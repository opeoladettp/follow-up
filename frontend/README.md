# FollowUpMedium Frontend

Modern, clean ChatGPT-like interface for the AI Newsroom.

## Features

- 🎨 Clean, modern UI with Tailwind CSS
- 💬 ChatGPT-style interface for story creation
- 📱 Fully responsive (mobile, tablet, desktop)
- ⚡ Fast and smooth animations
- 🎯 Intuitive UX with sidebar navigation
- 📊 Story timeline visualization
- 🎬 Video generation integration ready

## Tech Stack

- React 18
- Vite (fast build tool)
- Tailwind CSS (styling)
- Lucide React (icons)
- Axios (API calls)
- date-fns (date formatting)

## Getting Started

### Install Dependencies

```bash
cd frontend
npm install
```

### Run Development Server

```bash
npm run dev
```

The app will be available at http://localhost:4000

### Build for Production

```bash
npm run build
```

## Project Structure

```
frontend/
├── src/
│   ├── components/
│   │   ├── Sidebar.jsx          # Story list sidebar
│   │   ├── ChatInterface.jsx    # Main chat interface
│   │   └── StoryPanel.jsx       # Story details view
│   ├── services/
│   │   └── api.js               # API client
│   ├── App.jsx                  # Main app component
│   ├── main.jsx                 # Entry point
│   └── index.css                # Global styles
├── index.html
├── package.json
└── vite.config.js
```

## API Integration

The frontend connects to the backend API at `http://localhost:7070/api/v1`

Endpoints used:
- `GET /stories` - List all stories
- `POST /stories` - Create new story
- `GET /stories/:id/context` - Get story timeline

## Environment Variables

No environment variables needed for development. The Vite proxy handles API routing.

For production, set:
```
VITE_API_URL=https://your-backend-url.com
```

## Design System

### Colors
- Primary: Blue (#0ea5e9)
- Success: Green
- Warning: Yellow
- Error: Red

### Typography
- Font: System fonts (optimized for each OS)
- Sizes: Tailwind's default scale

### Components
- Rounded corners: 8-12px
- Shadows: Subtle, elevation-based
- Animations: Smooth, 200-300ms transitions

## Browser Support

- Chrome (latest)
- Firefox (latest)
- Safari (latest)
- Edge (latest)

## Contributing

1. Follow the existing code style
2. Use functional components with hooks
3. Keep components small and focused
4. Add comments for complex logic
