import { BoardListView } from "@/modules/board/ui/views/board-list-view";
import { createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute("/_authenticated/boards/")({
  component: RouteComponent,
});

function RouteComponent() {
  return <BoardListView />;
}
