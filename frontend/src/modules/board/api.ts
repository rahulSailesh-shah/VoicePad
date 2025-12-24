import { apiClient, ApiError } from "@/lib/api-client";
import type { CreateBoardResponse } from "./types";

const handleApiError = (errorMsg: string, status: number) => {
  throw new ApiError(errorMsg, status);
};

export const createBoard = async () => {
  const { data, error, status } = await apiClient.post<CreateBoardResponse>(
    "/boards",
    {}
  );

  if (error) {
    handleApiError(error, status);
  }

  return data;
};
