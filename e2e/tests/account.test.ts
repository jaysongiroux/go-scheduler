import { afterAll, describe, expect, test } from "vitest";
import {
  createAccount,
  deleteAccount,
  getAccount,
  getAccounts,
  updateAccount,
} from "../helpers/account";
import {
  AccountObject,
  ErrorObject,
  GenericPagedResponse,
  SuccessObject,
} from "../helpers/types";

describe("Account API", () => {
  const accounts: string[] = [];

  afterAll(async () => {
    for (const account of accounts) {
      const response = (await deleteAccount({
        accountId: account,
      })) as SuccessObject;
      expect(response.success).toBe(true);
    }
  });

  test("Should create an account", async () => {
    const accountUUID = crypto.randomUUID();
    const response = (await createAccount({
      accountId: accountUUID,
    })) as AccountObject;
    expect(response.account_id).toBe(accountUUID);
    accounts.push(response.account_id);
  });

  test("Should not create a duplicate account", async () => {
    const response = (await createAccount({
      accountId: accounts[0],
    })) as ErrorObject;
    expect(response.error).toBe("Account already exists");
    expect(response.details).toBe("Account already exists");
  });

  test("Should get an account", async () => {
    const response = (await getAccount({
      accountId: accounts[0],
    })) as AccountObject;
    expect(response.account_id).toBe(accounts[0]);
  });

  test("Should update an account", async () => {
    const response = (await updateAccount({
      accountId: accounts[0],
      settings: { edited: true },
      metadata: { test: "value" },
    })) as AccountObject;
    expect(response.account_id).toBe(accounts[0]);
    expect(response.settings).toEqual({ edited: true });
    expect(response.metadata).toEqual({ test: "value" });
  });

  test("Should not update a non-existent account", async () => {
    const response = (await updateAccount({
      accountId: "nonexistent-account",
      settings: { edited: true },
      metadata: { test: "value" },
    })) as ErrorObject;
    expect(response.error).toBe("Account not found");
    expect(response.details).toBe("Account not found");
  });

  test("Fetch accounts", async () => {
    const response = (await getAccounts({
      limit: 10,
      offset: 0,
      metadata: { test: "value" },
      settings: { edited: true },
    })) as GenericPagedResponse<AccountObject>;
    expect(response.count).toBe(1);
    expect(response.data[0].account_id).toBe(accounts[0]);
    expect(response.data[0].settings).toEqual({ edited: true });
    expect(response.data[0].metadata).toEqual({ test: "value" });

    const response2 = (await getAccounts({
      limit: 10,
      offset: 1,
      metadata: { test: "value" },
      settings: { edited: true },
    })) as GenericPagedResponse<AccountObject>;
    expect(response2.count).toBe(0);
    expect(response2.data).toEqual([]);
  });

  test("Should delete an account", async () => {
    const response = (await deleteAccount({
      accountId: accounts[0],
    })) as SuccessObject;
    expect(response.success).toBe(true);

    const response2 = (await getAccount({
      accountId: accounts[0],
    })) as ErrorObject;
    expect(response2.error).toBe("Account not found");

    // remove from accounts array
    accounts.splice(accounts.indexOf(accounts[0]), 1);

    // try to delete again
    const response3 = (await deleteAccount({
      accountId: accounts[0],
    })) as ErrorObject;
    expect(response3.error).toBe("Account not found");
    expect(response3.details).toBe("Account not found");
  });
});
