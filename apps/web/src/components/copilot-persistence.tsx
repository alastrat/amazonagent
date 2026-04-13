"use client";

import { useEffect, useRef } from "react";
import { useCopilotChat } from "@copilotkit/react-core";
import { TextMessage, MessageRole } from "@copilotkit/runtime-client-gql";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081";
const AUTH = "Bearer dev-user-dev-tenant";

interface StoredMessage {
  id: string;
  role: "user" | "assistant" | "system";
  content: string;
  created_at: string;
}

/**
 * Persists CopilotKit chat history to the Go backend DB.
 *
 * On mount: fetches /chat/history and replays messages via appendMessage
 * with followUp:false (so Claude is not re-triggered).
 *
 * On each completed exchange: posts the latest user/assistant pair to
 * /chat/save. Uses a watermark to avoid re-saving previously-restored messages.
 *
 * Mount once inside the CopilotKit provider.
 */
export function CopilotChatPersistence() {
  const { visibleMessages, appendMessage, isLoading } = useCopilotChat();
  const restoredRef = useRef(false);
  const savedMessageIdsRef = useRef<Set<string>>(new Set());

  // Restore messages from DB once on mount
  useEffect(() => {
    if (typeof window === "undefined") return;
    if (restoredRef.current) return;
    restoredRef.current = true;

    (async () => {
      try {
        const resp = await fetch(`${API_BASE}/chat/history`, {
          headers: { Authorization: AUTH },
        });
        if (!resp.ok) return;
        const data: { messages?: StoredMessage[] } = await resp.json();
        const messages = data.messages ?? [];
        for (const m of messages) {
          if (m.role === "system") continue;
          const role = m.role === "user" ? MessageRole.User : MessageRole.Assistant;
          savedMessageIdsRef.current.add(m.id);
          await appendMessage(
            new TextMessage({ id: m.id, role, content: m.content }),
            { followUp: false },
          );
        }
      } catch {
        // history load failed — start fresh
      }
    })();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Save messages after each complete exchange (when not streaming)
  useEffect(() => {
    if (typeof window === "undefined") return;
    if (isLoading) return;
    if (!visibleMessages || visibleMessages.length === 0) return;

    const textMessages = visibleMessages.filter(
      (m): m is TextMessage => m instanceof TextMessage,
    );

    // Find the most recent user → assistant pair that hasn't been saved yet
    let lastUser: TextMessage | null = null;
    let lastAssistant: TextMessage | null = null;
    for (const m of textMessages) {
      if (savedMessageIdsRef.current.has(m.id)) continue;
      if (m.role === MessageRole.User) {
        lastUser = m;
        lastAssistant = null;
      } else if (m.role === MessageRole.Assistant) {
        lastAssistant = m;
      }
    }

    // Only save if we have a complete user+assistant exchange
    if (!lastUser || !lastAssistant) return;

    const userContent = lastUser.content;
    const assistantContent = lastAssistant.content;
    if (!userContent || !assistantContent) return;

    // Mark as saved optimistically so we don't double-save
    savedMessageIdsRef.current.add(lastUser.id);
    savedMessageIdsRef.current.add(lastAssistant.id);

    fetch(`${API_BASE}/chat/save`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: AUTH,
      },
      body: JSON.stringify({
        user_message: userContent,
        assistant_message: assistantContent,
      }),
    }).catch(() => {
      // If save failed, allow retry next tick
      savedMessageIdsRef.current.delete(lastUser!.id);
      savedMessageIdsRef.current.delete(lastAssistant!.id);
    });
  }, [visibleMessages, isLoading]);

  return null;
}
