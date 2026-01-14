export const wait = (seconds: number) => {
  return new Promise((resolve) => setTimeout(resolve, seconds * 1000));
};

export const sleep = (ms: number) => {
  return new Promise((resolve) => setTimeout(resolve, ms));
};

export const API_BASE_URL = process.env.SCHEDULER_URL || "";

export const getHeaders = () => ({
  "api-key": process.env.SCHEDULER_API_KEY || "",
});
