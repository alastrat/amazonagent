"use client";

import { useEffect, useRef } from "react";
import { useCopilotMessagesContext } from "@copilotkit/react-core";
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
 * On mount: fetches /chat/history and writes directly into the
 * CopilotMessagesContext so messages render visibly in the sidebar.
 *
 * After each completed exchange: posts new user/assistant pairs to
 * /chat/save. Uses a watermark (savedMessageIdsRef) to avoid re-saving.
 *
 * Mount once inside the CopilotKit provider.
 */
export function CopilotChatPersistence() {
  const { messages, setMessages } = useCopilotMessagesContext();
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
        const stored = data.messages ?? [];
        if (stored.length === 0) return;

        const restored = stored
          .filter((m) => m.role !== "system")
          .map((m) => {
            savedMessageIdsRef.current.add(m.id);
            const role = m.role === "user" ? MessageRole.User : MessageRole.Assistant;
            return new TextMessage({
              id: m.id,
              role,
              content: m.content,
              createdAt: new Date(m.created_at),
            });
          });

        if (restored.length > 0) {
          setMessages(restored);
        }
      } catch {
        // history load failed — start fresh
      }
    })();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Save new user/assistant pairs when they appear in context
  useEffect(() => {
    if (typeof window === "undefined") return;
    if (!messages || messages.length === 0) return;

    const textMessages = messages.filter(
      (m): m is TextMessage => m instanceof TextMessage,
    );

    // Find the most recent unsaved user → assistant pair
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

    if (!lastUser || !lastAssistant) return;
    if (!lastUser.content || !lastAssistant.content) return;

    const userId = lastUser.id;
    const assistantId = lastAssistant.id;
    savedMessageIdsRef.current.add(userId);
    savedMessageIdsRef.current.add(assistantId);

    fetch(`${API_BASE}/chat/save`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: AUTH,
      },
      body: JSON.stringify({
        user_message: lastUser.content,
        assistant_message: lastAssistant.content,
      }),
    }).catch(() => {
      // Allow retry on next tick if save failed
      savedMessageIdsRef.current.delete(userId);
      savedMessageIdsRef.current.delete(assistantId);
    });
  }, [messages]);

  return null;
}
