import { apiClient, ApiError } from "@/lib/api-client";
import type {
  CreateBoardResponse,
  GetBoardResponse,
  GetBoardsByUserIDResponse,
  UpdateBoardRequest,
} from "./types";

const handleApiError = (errorMsg: string, status: number) => {
  throw new ApiError(errorMsg, status);
};

export const getBoard = async (id: string) => {
  const { data, error, status } = await apiClient.get<GetBoardResponse>(
    `/boards/${id}`
  );
  if (error) {
    handleApiError(error, status);
  }
  return data;
};

export const getBoards = async () => {
  const { data, error, status } =
    await apiClient.get<GetBoardsByUserIDResponse>("/boards");
  if (error) {
    handleApiError(error, status);
  }
  return data;
};

export const createBoard = async (name: string) => {
  const { data, error, status } = await apiClient.post<CreateBoardResponse>(
    "/boards",
    { name }
  );
  if (error) {
    handleApiError(error, status);
  }
  return data;
};

export const updateBoard = async (id: string, req: UpdateBoardRequest) => {
  const { data, error, status } = await apiClient.put<void>(`/boards/${id}`, {
    name: req.name,
    elements: req.elements,
  });
  if (error) {
    handleApiError(error, status);
  }
  return data;
};

export const deleteBoard = async (id: string) => {
  const { data, error, status } = await apiClient.delete(
    `/boards/${id}` as string
  );
  if (error) {
    handleApiError(error, status);
  }
  return data;
};
