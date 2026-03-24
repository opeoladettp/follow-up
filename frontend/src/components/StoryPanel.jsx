import { useState, useEffect } from 'react'
import { X, Calendar, ExternalLink, Clock, Video, FileText, Image as ImageIcon, Play, Loader2 } from 'lucide-react'
import { format } from 'date-fns'
import { api } from '../services/api'

export default function StoryPanel({ story, onClose }) {
  const [report, setReport] = useState(null)
  const [loadingReport, setLoadingReport] = useState(true)

  const developments = story.context?.developments || []
  const script = story.description || ''

  useEffect(() => {
    const fetchReport = async () => {
      try {
        setLoadingReport(true)
        const data = await api.getReportByTitle(story.title)
        setReport(data)
      } catch {
        // no linked report found — that's fine
      } finally {
        setLoadingReport(false)
      }
    }
    if (story.title) fetchReport()
  }, [story.title])

  const videoURL = report?.video_url || ''
  const images = report?.images || []

  return (
    <div className="flex-1 flex flex-col h-full bg-white dark:bg-gray-900">
      {/* Header */}
      <div className="border-b border-gray-200 dark:border-gray-700 px-6 py-4 flex-shrink-0">
        <div className="flex items-start justify-between gap-4">
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 mb-2">
              <span className="px-2 py-1 bg-green-100 text-green-700 text-xs font-medium rounded">
                {story.status || 'Active'}
              </span>
              {story.created_at && (
                <span className="text-xs text-gray-500">
                  {format(new Date(story.created_at), 'MMM d, yyyy')}
                </span>
              )}
            </div>
            <h1 className="text-xl font-bold text-gray-900 dark:text-white">{story.title}</h1>
            {report?.author && (
              <p className="text-sm text-gray-500 mt-1">By {report.author}</p>
            )}
          </div>
          <button
            onClick={onClose}
            className="p-2 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg transition-colors flex-shrink-0"
          >
            <X className="w-5 h-5 text-gray-500" />
          </button>
        </div>
      </div>

      {/* Scrollable content */}
      <div className="flex-1 overflow-y-auto px-6 py-6 space-y-8">

        {/* Full Script */}
        {script && (
          <section>
            <h2 className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider mb-3 flex items-center gap-2">
              <FileText className="w-4 h-4" /> Script
            </h2>
            <div className="bg-gray-50 dark:bg-gray-800 rounded-lg p-4 text-sm text-gray-800 dark:text-gray-200 whitespace-pre-wrap leading-relaxed">
              {script}
            </div>
          </section>
        )}

        {/* Media loading indicator */}
        {loadingReport && (
          <div className="flex items-center gap-2 text-sm text-gray-400">
            <Loader2 className="w-4 h-4 animate-spin" />
            Loading media assets...
          </div>
        )}

        {/* Video */}
        {videoURL && (
          <section>
            <h2 className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider mb-3 flex items-center gap-2">
              <Play className="w-4 h-4" /> Generated Video
            </h2>
            <video
              src={videoURL}
              controls
              className="w-full rounded-lg border border-gray-200 dark:border-gray-700 max-h-80"
            />
          </section>
        )}

        {/* Images */}
        {images.length > 0 && (
          <section>
            <h2 className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider mb-3 flex items-center gap-2">
              <ImageIcon className="w-4 h-4" /> Story Images
            </h2>
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
              {images.map((img, i) => (
                <a key={i} href={img.url || img} target="_blank" rel="noopener noreferrer">
                  <img
                    src={img.url || img}
                    alt={`Story image ${i + 1}`}
                    className="w-full h-36 object-cover rounded-lg border border-gray-200 dark:border-gray-700 hover:opacity-90 transition-opacity"
                  />
                </a>
              ))}
            </div>
          </section>
        )}

        {/* Timeline */}
        {developments.length > 0 && (
          <section>
            <h2 className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider mb-3 flex items-center gap-2">
              <Calendar className="w-4 h-4" /> Timeline
            </h2>
            <div className="space-y-4">
              {developments.map((dev, index) => (
                <div key={dev.id || index} className="relative pl-6 pb-6 border-l-2 border-gray-200 dark:border-gray-700 last:border-l-0 last:pb-0">
                  <div className="absolute left-0 top-0 -translate-x-1/2 w-3 h-3 bg-brand-primary rounded-full border-2 border-white dark:border-gray-900" />
                  <div className="bg-gray-50 dark:bg-gray-800 rounded-lg p-3">
                    <div className="flex items-center justify-between mb-1">
                      <span className={`px-2 py-0.5 text-xs font-medium rounded ${
                        dev.type === 'breaking' ? 'bg-red-100 text-red-700' : 'bg-blue-100 text-blue-700'
                      }`}>
                        {dev.type || 'Update'}
                      </span>
                      {dev.timestamp && (
                        <span className="text-xs text-gray-500">
                          {format(new Date(dev.timestamp), 'MMM d, h:mm a')}
                        </span>
                      )}
                    </div>
                    <p className="text-sm text-gray-800 dark:text-gray-200">{dev.content || dev.summary}</p>
                    {dev.source?.url && (
                      <a href={dev.source.url} target="_blank" rel="noopener noreferrer"
                        className="inline-flex items-center gap-1 text-xs text-brand-primary hover:underline mt-1">
                        View source <ExternalLink className="w-3 h-3" />
                      </a>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </section>
        )}

        {/* Empty state — only show after loading is done */}
        {!loadingReport && !script && !videoURL && images.length === 0 && developments.length === 0 && (
          <div className="text-center py-16 text-gray-400">
            <FileText className="w-12 h-12 mx-auto mb-3 opacity-40" />
            <p className="text-sm">No content available for this story</p>
          </div>
        )}
      </div>
    </div>
  )
}
