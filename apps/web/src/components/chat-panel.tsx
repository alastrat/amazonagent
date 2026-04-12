"use client";

import { useState, useRef, useEffect } from "react";
import ReactMarkdown from "react-markdown";
import { Button } from "@/components/ui/button";
import { useChat } from "@/hooks/use-chat";
import type { ChatMessage } from "@/lib/types";

interface Props {
  isOpen: boolean;
  onToggle: () => void;
}

function ChatBubble({ message }: { message: ChatMessage }) {
  const isUser = message.role === "user";
  return (
    <div className={`flex ${isUser ? "justify-end" : "justify-start"} mb-3`}>
      <div
        className={`max-w-[85%] rounded-lg px-3 py-2 text-sm ${
          isUser
            ? "bg-primary text-primary-foreground"
            : "bg-muted text-foreground"
        }`}
      >
        {isUser ? (
          <p className="whitespace-pre-wrap">{message.content}</p>
        ) : (
          <div className="prose prose-sm dark:prose-invert max-w-none [&>p]:mb-1 [&>p:last-child]:mb-0">
            <ReactMarkdown>{message.content}</ReactMarkdown>
          </div>
        )}
      </div>
    </div>
  );
}

export function ChatPanel({ isOpen, onToggle }: Props) {
  const { messages, isLoading, isConnected, error, sendMessage } = useChat(isOpen);
  const [input, setInput] = useState("");
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);

  // Auto-scroll to bottom when new messages arrive
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages.length]);

  // Focus input when panel opens
  useEffect(() => {
    if (isOpen) {
      setTimeout(() => inputRef.current?.focus(), 100);
    }
  }, [isOpen]);

  function handleSend() {
    if (!input.trim() || isLoading) return;
    sendMessage(input);
    setInput("");
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  }

  if (!isOpen) return null;

  return (
    <div className="fixed right-0 top-0 h-full w-[400px] border-l bg-background flex flex-col z-50 shadow-xl">
      {/* Header */}
      <div className="flex items-center justify-between border-b px-4 py-3">
        <div className="flex items-center gap-2">
          <div
            className={`h-2 w-2 rounded-full ${isConnected ? "bg-green-500" : "bg-yellow-500"}`}
          />
          <h3 className="font-semibold text-sm">FBA Concierge</h3>
        </div>
        <Button variant="ghost" size="sm" onClick={onToggle}>
          &times;
        </Button>
      </div>

      {/* Messages */}
      <div className="flex-1 overflow-y-auto px-4 py-3">
        {messages.length === 0 && !isLoading && (
          <div className="text-center text-sm text-muted-foreground mt-8">
            <p className="font-medium mb-1">Hi! I&apos;m your FBA Concierge.</p>
            <p>Ask me about your products, eligibility, categories, or strategy.</p>
          </div>
        )}
        {messages.map((msg) => (
          <ChatBubble key={msg.id} message={msg} />
        ))}
        {isLoading && (
          <div className="flex justify-start mb-3">
            <div className="bg-muted rounded-lg px-3 py-2 text-sm text-muted-foreground">
              <span className="animate-pulse">Thinking...</span>
            </div>
          </div>
        )}
        {error && (
          <div className="flex justify-center mb-3">
            <div className="bg-red-50 dark:bg-red-950 text-red-600 dark:text-red-400 rounded-lg px-3 py-2 text-xs">
              {error}
            </div>
          </div>
        )}
        <div ref={messagesEndRef} />
      </div>

      {/* Input */}
      <div className="border-t px-4 py-3">
        <div className="flex gap-2">
          <textarea
            ref={inputRef}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Ask your concierge..."
            rows={1}
            className="flex-1 resize-none rounded-lg border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-primary"
          />
          <Button
            size="sm"
            onClick={handleSend}
            disabled={!input.trim() || isLoading}
          >
            Send
          </Button>
        </div>
        <p className="text-[10px] text-muted-foreground mt-1">
          Shift+Enter for new line
        </p>
      </div>
    </div>
  );
}
