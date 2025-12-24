import { BoardRoomView } from "@/modules/board/ui/views/board-room-view";
import { createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute("/_authenticated/boards/$boardId")({
  component: RouteComponent,
});

function RouteComponent() {
  const { boardId } = Route.useParams();
  return <BoardRoomView boardId={boardId} />;
}
