import ReactMarkdown from "react-markdown";

import { cn } from "@/lib/cn";

// Shared renderer for problem statements — one source of markdown styling for
// the contest room and the admin authoring previews.
export function Markdown({ children, className }: { children: string; className?: string }) {
  return (
    <div className={cn("text-sm leading-relaxed text-muted", className)}>
      <ReactMarkdown
        components={{
          h1: (props) => (
            <h3 className="mt-4 mb-2 font-display font-semibold text-foreground" {...props} />
          ),
          h2: (props) => (
            <h4 className="mt-4 mb-2 font-display font-semibold text-foreground" {...props} />
          ),
          strong: (props) => <strong className="text-foreground" {...props} />,
          p: (props) => <p className="mb-3" {...props} />,
          ul: (props) => <ul className="mb-3 list-disc pl-5" {...props} />,
          ol: (props) => <ol className="mb-3 list-decimal pl-5" {...props} />,
          code: (props) => (
            <code
              className="rounded bg-surface-2 px-1.5 py-0.5 font-mono text-xs text-primary"
              {...props}
            />
          ),
        }}
      >
        {children}
      </ReactMarkdown>
    </div>
  );
}
