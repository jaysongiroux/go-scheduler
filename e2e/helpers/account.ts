import { SuccessObject } from "./types";

export const deleteAccount = async ({ accountId }: { accountId: string }) => {
  const headers = {
    "api-key": process.env.SCHEDULER_API_KEY || "",
  };
  const response = await fetch(
    `${process.env.SCHEDULER_URL}/api/v1/accounts?account_id=${accountId}`,
    {
      method: "DELETE",
      headers,
    }
  );
  return response.json() as Promise<SuccessObject>;
};
