import {
  AccountObject,
  ErrorObject,
  GenericPagedResponse,
  SuccessObject,
} from "./types";

export const createAccount = async ({
  accountId,
  settings,
  metadata,
}: {
  accountId: string;
  settings?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
}) => {
  const body = {
    account_id: accountId,
    ...(settings && { settings }),
    ...(metadata && { metadata }),
  };
  const headers = {
    "api-key": process.env.SCHEDULER_API_KEY || "",
  };
  const response = await fetch(`${process.env.SCHEDULER_URL}/api/v1/accounts`, {
    method: "POST",
    headers,
    body: JSON.stringify(body),
  });

  return response.json() as Promise<AccountObject | ErrorObject>;
};

export const deleteAccount = async ({ accountId }: { accountId: string }) => {
  const headers = {
    "api-key": process.env.SCHEDULER_API_KEY || "",
  };
  const response = await fetch(
    `${process.env.SCHEDULER_URL}/api/v1/accounts/${accountId}`,
    {
      method: "DELETE",
      headers,
    }
  );

  return response.json() as Promise<SuccessObject | ErrorObject>;
};

export const updateAccount = async ({
  accountId,
  settings,
  metadata,
}: {
  accountId: string;
  settings?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
}) => {
  const body = {
    ...(settings && { settings }),
    ...(metadata && { metadata }),
  };
  const headers = {
    "api-key": process.env.SCHEDULER_API_KEY || "",
  };
  const response = await fetch(
    `${process.env.SCHEDULER_URL}/api/v1/accounts/${accountId}`,
    {
      method: "PUT",
      headers,
      body: JSON.stringify(body),
    }
  );

  return response.json() as Promise<AccountObject | ErrorObject>;
};

export const getAccount = async ({ accountId }: { accountId: string }) => {
  const headers = {
    "api-key": process.env.SCHEDULER_API_KEY || "",
  };
  const response = await fetch(
    `${process.env.SCHEDULER_URL}/api/v1/accounts/${accountId}`,
    { method: "GET", headers }
  );
  return response.json() as Promise<AccountObject | ErrorObject>;
};

export const getAccounts = async ({
  limit,
  offset,
  metadata,
  settings,
}: {
  limit: number;
  offset: number;
  metadata?: Record<string, unknown>;
  settings?: Record<string, unknown>;
}) => {
  const headers = {
    "api-key": process.env.SCHEDULER_API_KEY || "",
  };
  const response = await fetch(
    `${
      process.env.SCHEDULER_URL
    }/api/v1/accounts?limit=${limit}&offset=${offset}&metadata=${JSON.stringify(
      metadata
    )}&settings=${JSON.stringify(settings)}`,
    { method: "GET", headers }
  );
  return response.json() as Promise<
    GenericPagedResponse<AccountObject> | ErrorObject
  >;
};
