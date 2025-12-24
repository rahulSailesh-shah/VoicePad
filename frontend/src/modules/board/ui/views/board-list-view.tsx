import { Button } from "@/components/ui/button";
import { useMutationCreateBoard } from "../../hooks/use-board";
import { useNavigate } from "@tanstack/react-router";

export const BoardListView = () => {
  const createBoard = useMutationCreateBoard();
  const navigate = useNavigate();

  const handleCreateBoard = () => {
    createBoard.mutate(undefined, {
      onSuccess: (data) => {
        if (!data?.token || !data?.id) {
          console.error("No token received from server");
          return;
        }
        sessionStorage.setItem(`room-token`, data.token);
        navigate({
          to: "/boards/$boardId",
          params: { boardId: data.id },
        });
      },
      onError: (error) => {
        console.error("Failed to start meeting:", error);
      },
    });
  };

  return (
    <div className="flex-1 flex justify-center">
      <Button onClick={handleCreateBoard}>Create board</Button>
    </div>
  );
};
