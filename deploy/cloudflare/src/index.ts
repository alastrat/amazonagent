import { Container } from "cloudflare:workers";

export class APIContainer extends Container {
  defaultPort = 8080;
  sleepAfter = "5m";

  override onStart(): void {
    console.log("FBA API container started");
  }

  override onStop(): void {
    console.log("FBA API container stopped");
  }

  override onError(error: unknown): void {
    console.error("FBA API container error:", error);
  }
}

interface Env {
  API_CONTAINER: DurableObjectNamespace<APIContainer>;
}

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    // Route all requests to a single API container instance
    const id = env.API_CONTAINER.idFromName("api");
    const container = env.API_CONTAINER.get(id);
    return await container.fetch(request);
  },
};
