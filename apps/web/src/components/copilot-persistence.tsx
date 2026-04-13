"use client";

import { useEffect, useRef } from "react";
import { useCopilotChat } from "@copilotkit/react-core";
import { TextMessage, MessageRole } from "@copilotkit/runtime-client-gql";

const STORAGE_KEY = "copilot-chat-history-v1";

/**
 * Persists CopilotKit chat messages to localStorage and restores them
 * on page load. Mount this inside the CopilotKit provider.
 */
export function CopilotChatPersistence() {
  const { visibleMessages, appendMessage, isLoading } = useCopilotChat();
  const restoredRef = useRef(false);

  // Restore messages from localStorage once on mount
  useEffect(() => {
    if (typeof window === "undefined") return;
    if (restoredRef.current) return;
    restoredRef.current = true;

    const saved = localStorage.getItem(STORAGE_KEY);
    if (!saved) return;

    try {
      const parsed: Array<{ role: string; content: string; id: string }> = JSON.parse(saved);
      // Replay messages without triggering LLM (followUp: false)
      (async () => {
        for (const m of parsed) {
          const role = m.role === "user" ? MessageRole.User : MessageRole.Assistant;
          await appendMessage(
            new TextMessage({ id: m.id, role, content: m.content }),
            { followUp: false },
          );
        }
      })();
    } catch {
      // corrupted storage — clear it
      localStorage.removeItem(STORAGE_KEY);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Save messages when not streaming
  useEffect(() => {
    if (typeof window === "undefined") return;
    if (isLoading) return;
    if (visibleMessages.length === 0) return;

    const serializable = visibleMessages
      .filter((m): m is TextMessage => m instanceof TextMessage)
      .map((msg) => ({
        id: msg.id,
        role: msg.role === MessageRole.User ? "user" : "assistant",
        content: msg.content,
      }));

    if (serializable.length > 0) {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(serializable));
    }
  }, [visibleMessages, isLoading]);

  return null;
}
