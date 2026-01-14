import { HttpClient } from "../utils/http";
import {
  Webhook,
  CreateWebhookRequest,
  UpdateWebhookRequest,
  WebhookDelivery,
  PagedResponse,
} from "../types";

export class Webhooks {
  private http: HttpClient;

  constructor(http: HttpClient) {
    this.http = http;
  }

  async create(data: CreateWebhookRequest): Promise<Webhook> {
    return this.http.post<Webhook>("/api/v1/webhooks", data);
  }

  async get(webhookUID: string): Promise<Webhook> {
    return this.http.get<Webhook>(`/api/v1/webhooks/${webhookUID}`);
  }

  async list(limit: number = 50, offset: number = 0): Promise<PagedResponse<Webhook>> {
    return this.http.get<PagedResponse<Webhook>>("/api/v1/webhooks", {
      params: { limit, offset },
    });
  }

  async update(webhookUID: string, data: UpdateWebhookRequest): Promise<Webhook> {
    return this.http.put<Webhook>(`/api/v1/webhooks/${webhookUID}`, data);
  }

  async delete(webhookUID: string): Promise<void> {
    return this.http.delete<void>(`/api/v1/webhooks/${webhookUID}`);
  }

  async getDeliveries(
    webhookUID: string,
    limit: number = 50,
    offset: number = 0
  ): Promise<PagedResponse<WebhookDelivery>> {
    return this.http.get<PagedResponse<WebhookDelivery>>(
      `/api/v1/webhooks/deliveries/${webhookUID}`,
      {
        params: { limit, offset },
      }
    );
  }
}
