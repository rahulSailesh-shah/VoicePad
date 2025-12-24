import { useMutation, useQueryClient } from "@tanstack/react-query";
import { createBoard } from "../api";

// Query hooks

// Mutation hooks
export const useMutationCreateBoard = () => {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => createBoard(),
    onSuccess: (data) => {
      queryClient.invalidateQueries({
        queryKey: ["board", data?.id],
      });
    },
    onError: () => {},
  });
};
