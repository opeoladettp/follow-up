package services

import (
	"context"
	"fmt"
	"html"
	"time"

	"followupmedium-newsroom/internal/database"
	"followupmedium-newsroom/internal/models"

	"github.com/mmcdole/gofeed"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type RSSService struct {
	db       *database.MongoDB
	parser   *gofeed.Parser
	rssFeeds []string // seed feeds from env (read-only fallback)
}

type Headline struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	URL         string    `json:"url"`
	Source      string    `json:"source"`
	Category    string    `json:"category"`
	PublishedAt time.Time `json:"published_at"`
	ImageURL    string    `json:"image_url,omitempty"`
}

func NewRSSService(db *database.MongoDB, rssFeeds []string) *RSSService {
	svc := &RSSService{
		db:       db,
		parser:   gofeed.NewParser(),
		rssFeeds: rssFeeds,
	}
	// Seed default feeds into DB if collection is empty
	svc.seedDefaultFeeds()
	return svc
}

func (r *RSSService) feedsCollection() *mongo.Collection {
	return r.db.Database.Collection("rss_feeds")
}

// seedDefaultFeeds inserts env-configured feeds into DB if none exist yet
func (r *RSSService) seedDefaultFeeds() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	count, err := r.feedsCollection().CountDocuments(ctx, bson.M{})
	if err != nil || count > 0 {
		return
	}

	for i, feedURL := range r.rssFeeds {
		feed := models.RSSFeed{
			ID:        primitive.NewObjectID(),
			Name:      fmt.Sprintf("Feed %d", i+1),
			URL:       feedURL,
			Category:  "General",
			Active:    true,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		_, _ = r.feedsCollection().InsertOne(ctx, feed)
	}
	logrus.Infof("Seeded %d default RSS feeds into DB", len(r.rssFeeds))
}

// GetRSSFeeds returns all feeds from MongoDB
func (r *RSSService) GetRSSFeeds() ([]models.RSSFeed, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := r.feedsCollection().Find(ctx, bson.M{}, options.Find().SetSort(bson.M{"created_at": 1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var feeds []models.RSSFeed
	if err = cursor.All(ctx, &feeds); err != nil {
		return nil, err
	}
	return feeds, nil
}

// AddRSSFeed persists a new feed to MongoDB
func (r *RSSService) AddRSSFeed(feedURL, feedName, category string) (*models.RSSFeed, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check duplicate
	var existing models.RSSFeed
	err := r.feedsCollection().FindOne(ctx, bson.M{"url": feedURL}).Decode(&existing)
	if err == nil {
		return nil, fmt.Errorf("feed already exists")
	}

	if category == "" {
		category = "General"
	}

	feed := models.RSSFeed{
		ID:        primitive.NewObjectID(),
		Name:      feedName,
		URL:       feedURL,
		Category:  category,
		Active:    true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err = r.feedsCollection().InsertOne(ctx, feed)
	if err != nil {
		return nil, err
	}

	logrus.WithFields(logrus.Fields{"url": feedURL, "name": feedName}).Info("RSS feed added")
	return &feed, nil
}

// UpdateRSSFeed updates name/category/active for a feed
func (r *RSSService) UpdateRSSFeed(feedID, name, category string, active *bool) (*models.RSSFeed, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	objID, err := primitive.ObjectIDFromHex(feedID)
	if err != nil {
		return nil, fmt.Errorf("invalid feed ID")
	}

	update := bson.M{"updated_at": time.Now()}
	if name != "" {
		update["name"] = name
	}
	if category != "" {
		update["category"] = category
	}
	if active != nil {
		update["active"] = *active
	}

	after := options.After
	var updated models.RSSFeed
	err = r.feedsCollection().FindOneAndUpdate(
		ctx,
		bson.M{"_id": objID},
		bson.M{"$set": update},
		options.FindOneAndUpdate().SetReturnDocument(after),
	).Decode(&updated)
	if err != nil {
		return nil, fmt.Errorf("feed not found")
	}

	return &updated, nil
}

// DeleteRSSFeed removes a feed from MongoDB
func (r *RSSService) DeleteRSSFeed(feedID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	objID, err := primitive.ObjectIDFromHex(feedID)
	if err != nil {
		return fmt.Errorf("invalid feed ID")
	}

	result, err := r.feedsCollection().DeleteOne(ctx, bson.M{"_id": objID})
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return fmt.Errorf("feed not found")
	}
	return nil
}

// FetchAllHeadlines fetches from all active DB feeds
func (r *RSSService) FetchAllHeadlines() ([]Headline, error) {
	feeds, err := r.GetRSSFeeds()
	if err != nil || len(feeds) == 0 {
		// Fallback to env feeds
		return r.fetchFromURLs(r.rssFeeds, map[string]string{})
	}

	urls := make([]string, 0, len(feeds))
	categories := make(map[string]string)
	for _, f := range feeds {
		if f.Active {
			urls = append(urls, f.URL)
			categories[f.URL] = f.Category
		}
	}
	return r.fetchFromURLs(urls, categories)
}

func (r *RSSService) fetchFromURLs(urls []string, categories map[string]string) ([]Headline, error) {
	var allHeadlines []Headline
	for _, feedURL := range urls {
		headlines, err := r.fetchHeadlinesFromFeed(feedURL, categories[feedURL])
		if err != nil {
			logrus.WithError(err).WithField("feed", feedURL).Error("Failed to fetch headlines")
			continue
		}
		allHeadlines = append(allHeadlines, headlines...)
	}
	logrus.WithField("count", len(allHeadlines)).Info("Fetched RSS headlines")
	return allHeadlines, nil
}

// FetchHeadlinesBySource fetches headlines from a specific source
func (r *RSSService) FetchHeadlinesBySource(source string) ([]Headline, error) {
	feeds, _ := r.GetRSSFeeds()
	for _, f := range feeds {
		if contains(f.URL, source) || contains(f.Name, source) {
			return r.fetchHeadlinesFromFeed(f.URL, f.Category)
		}
	}
	// Fallback to env feeds
	for _, url := range r.rssFeeds {
		if contains(url, source) {
			return r.fetchHeadlinesFromFeed(url, "")
		}
	}
	return nil, fmt.Errorf("source not found: %s", source)
}

func (r *RSSService) fetchHeadlinesFromFeed(feedURL, category string) ([]Headline, error) {
	feed, err := r.parser.ParseURL(feedURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse feed: %w", err)
	}

	headlines := make([]Headline, 0, len(feed.Items))
	for _, item := range feed.Items {
		headline := Headline{
			ID:          generateHeadlineID(item),
			Title:       html.UnescapeString(item.Title),
			Description: html.UnescapeString(item.Description),
			URL:         item.Link,
			Source:      html.UnescapeString(feed.Title),
			Category:    category,
		}
		if item.PublishedParsed != nil {
			headline.PublishedAt = *item.PublishedParsed
		}
		if item.Image != nil {
			headline.ImageURL = item.Image.URL
		}
		headlines = append(headlines, headline)
	}
	return headlines, nil
}

// SaveReport saves a correspondent's edited report and creates a Story entry
func (r *RSSService) SaveReport(headlineID, title, script, author string) (string, error) {
	report := models.NewsReport{
		ID:         primitive.NewObjectID(),
		HeadlineID: headlineID,
		Title:      title,
		Script:     script,
		Author:     author,
		Status:     "draft",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	_, err := r.db.NewsReports().InsertOne(nil, report)
	if err != nil {
		return "", fmt.Errorf("failed to save report: %w", err)
	}

	story := models.Story{
		ID:          primitive.NewObjectID(),
		Title:       title,
		Description: script,
		Category:    "news-report",
		Tags:        []string{"rss", "correspondent"},
		Sources: []models.Source{
			{Type: "rss", URL: headlineID, Name: author},
		},
		Status:    "active",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err = r.db.Stories().InsertOne(nil, story)
	if err != nil {
		logrus.WithError(err).Warn("Failed to create Story entry, but report was saved")
	}

	return report.ID.Hex(), nil
}

func generateHeadlineID(item *gofeed.Item) string {
	if item.GUID != "" {
		return item.GUID
	}
	return item.Link
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(substr) > 0 && len(s) > 0 &&
			(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr)))
}

func (r *RSSService) UpdateReportVideoStatus(reportID, videoJobID, status, videoURL string) error {
	objID, err := primitive.ObjectIDFromHex(reportID)
	if err != nil {
		return fmt.Errorf("invalid report ID: %w", err)
	}

	update := map[string]interface{}{
		"video_job_id": videoJobID,
		"video_status": status,
		"updated_at":   time.Now(),
	}
	if videoURL != "" {
		update["video_url"] = videoURL
	}

	_, err = r.db.NewsReports().UpdateOne(
		nil,
		map[string]interface{}{"_id": objID},
		map[string]interface{}{"$set": update},
	)
	return err
}

func (r *RSSService) GetReportStatus(reportID string) (*models.NewsReport, error) {
	objID, err := primitive.ObjectIDFromHex(reportID)
	if err != nil {
		return nil, fmt.Errorf("invalid report ID: %w", err)
	}

	var report models.NewsReport
	err = r.db.NewsReports().FindOne(nil, map[string]interface{}{"_id": objID}).Decode(&report)
	if err != nil {
		return nil, fmt.Errorf("report not found: %w", err)
	}
	return &report, nil
}
