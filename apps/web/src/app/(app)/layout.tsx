"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { CopilotKit } from "@copilotkit/react-core";
import "@copilotkit/react-ui/styles.css";
import { useState } from "react";
import { useRouter } from "next/navigation";
import { AppShell } from "@/components/app-shell";
import { CopilotToolRenderers } from "@/components/copilot-tools";
import { CopilotChatPersistence } from "@/components/copilot-persistence";
import { AuthProvider, useAuth } from "@/lib/auth-provider";

function AuthGate({ children }: { children: React.ReactNode }) {
  const { user, loading } = useAuth();
  const router = useRouter();

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p className="text-gray-500">Loading...</p>
      </div>
    );
  }

  if (!user) {
    router.push("/login");
    return null;
  }

  return <AppShell>{children}</AppShell>;
}

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const [queryClient] = useState(() => new QueryClient());

  return (
    <AuthProvider>
      <QueryClientProvider client={queryClient}>
        <CopilotKit runtimeUrl="/api/copilotkit" showDevConsole={false}>
          <CopilotToolRenderers />
          <CopilotChatPersistence />
          <AuthGate>{children}</AuthGate>
        </CopilotKit>
      </QueryClientProvider>
    </AuthProvider>
  );
}
