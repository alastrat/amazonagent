"use client";

import { useEffect, useReducer, useCallback, useRef } from "react";
import { apiClient } from "@/lib/api-client";
import type { ChatMessage, ChatEvent } from "@/lib/types";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8081";

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

export interface ChatState {
  messages: ChatMessage[];
  isLoading: boolean;
  isConnected: boolean;
  error: string | null;
}

const initialState: ChatState = {
  messages: [],
  isLoading: false,
  isConnected: false,
  error: null,
};

type Action =
  | { type: "CONNECTED" }
  | { type: "DISCONNECTED" }
  | { type: "LOAD_HISTORY"; messages: ChatMessage[] }
  | { type: "ADD_USER_MESSAGE"; message: ChatMessage }
  | { type: "ADD_ASSISTANT_MESSAGE"; message: ChatMessage }
  | { type: "SET_TYPING"; typing: boolean }
  | { type: "SET_ERROR"; error: string }
  | { type: "CLEAR_ERROR" };

function chatReducer(state: ChatState, action: Action): ChatState {
  switch (action.type) {
    case "CONNECTED":
      return { ...state, isConnected: true };
    case "DISCONNECTED":
      return { ...state, isConnected: false };
    case "LOAD_HISTORY":
      return { ...state, messages: action.messages };
    case "ADD_USER_MESSAGE":
      return {
        ...state,
        messages: [...state.messages, action.message],
        isLoading: true,
        error: null,
      };
    case "ADD_ASSISTANT_MESSAGE":
      return {
        ...state,
        messages: [...state.messages, action.message],
        isLoading: false,
      };
    case "SET_TYPING":
      return { ...state, isLoading: action.typing };
    case "SET_ERROR":
      return { ...state, error: action.error, isLoading: false };
    case "CLEAR_ERROR":
      return { ...state, error: null };
    default:
      return state;
  }
}

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

export function useChat(enabled: boolean) {
  const [state, dispatch] = useReducer(chatReducer, initialState);
  const esRef = useRef<EventSource | null>(null);

  // Load chat history on mount
  useEffect(() => {
    if (!enabled) return;
    apiClient
      .getChatHistory()
      .then((data) => {
        dispatch({ type: "LOAD_HISTORY", messages: data.messages || [] });
      })
      .catch(() => {
        // No history yet — that's fine
      });
  }, [enabled]);

  // Connect to SSE stream
  useEffect(() => {
    if (!enabled) return;

    const token = apiClient.getToken();
    const url = `${API_BASE}/chat/events${token ? `?token=${token}` : ""}`;
    const es = new EventSource(url);
    esRef.current = es;

    es.onopen = () => {
      dispatch({ type: "CONNECTED" });
      // Reload history on reconnect — catches messages received while SSE was down
      apiClient.getChatHistory().then((data) => {
        if (data.messages && data.messages.length > 0) {
          dispatch({ type: "LOAD_HISTORY", messages: data.messages });
        }
      }).catch(() => {});
    };

    es.addEventListener("message", (e) => {
      const evt: ChatEvent = JSON.parse((e as MessageEvent).data);
      const d = evt.data;
      dispatch({
        type: "ADD_ASSISTANT_MESSAGE",
        message: {
          id: (d.id as string) || crypto.randomUUID(),
          tenant_id: "",
          session_id: "",
          role: "assistant",
          content: (d.content as string) || "",
          created_at: evt.timestamp,
        },
      });
    });

    es.addEventListener("typing", () => {
      dispatch({ type: "SET_TYPING", typing: true });
    });

    es.addEventListener("done", () => {
      dispatch({ type: "SET_TYPING", typing: false });
    });

    es.addEventListener("error", (e) => {
      const evt: ChatEvent = JSON.parse((e as MessageEvent).data);
      dispatch({ type: "SET_ERROR", error: (evt.data.error as string) || "Unknown error" });
    });

    es.onerror = () => {
      dispatch({ type: "DISCONNECTED" });
      // EventSource auto-reconnects
    };

    return () => {
      es.close();
      esRef.current = null;
    };
  }, [enabled]);

  // Send message
  const sendMessage = useCallback(
    async (content: string) => {
      if (!content.trim()) return;

      // Optimistic: add user message immediately
      dispatch({
        type: "ADD_USER_MESSAGE",
        message: {
          id: crypto.randomUUID(),
          tenant_id: "",
          session_id: "",
          role: "user",
          content: content.trim(),
          created_at: new Date().toISOString(),
        },
      });

      try {
        await apiClient.sendChatMessage(content.trim());
      } catch (err) {
        dispatch({
          type: "SET_ERROR",
          error: err instanceof Error ? err.message : "Failed to send message",
        });
      }
    },
    [],
  );

  return {
    ...state,
    sendMessage,
  };
}
