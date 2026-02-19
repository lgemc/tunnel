import { useEffect, useRef } from 'react'
import type { LogEvent } from '../api/client'

interface LogViewerProps {
  events: LogEvent[]
  loading?: boolean
}

export default function LogViewer({ events, loading }: LogViewerProps) {
  const bottomRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [events])

  if (loading) {
    return (
      <div className="h-72 bg-gray-950 rounded-lg border border-gray-800 flex items-center justify-center">
        <span className="text-sm text-gray-500 animate-pulse">Loading logs…</span>
      </div>
    )
  }

  if (events.length === 0) {
    return (
      <div className="h-72 bg-gray-950 rounded-lg border border-gray-800 flex items-center justify-center">
        <span className="text-sm text-gray-600">No log events in the last 30 minutes</span>
      </div>
    )
  }

  return (
    <div className="h-72 bg-gray-950 rounded-lg border border-gray-800 overflow-auto font-mono text-xs p-3 space-y-0.5">
      {events.map((e, i) => (
        <div key={i} className="flex gap-3 group hover:bg-gray-900 rounded px-1 py-0.5">
          <span className="text-gray-600 shrink-0 select-none">
            {new Date(e.timestamp).toLocaleTimeString()}
          </span>
          <span className="text-gray-300 break-all whitespace-pre-wrap">{e.message.trim()}</span>
        </div>
      ))}
      <div ref={bottomRef} />
    </div>
  )
}
