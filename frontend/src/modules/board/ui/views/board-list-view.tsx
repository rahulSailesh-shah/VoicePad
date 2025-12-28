import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  useMutationCreateBoard,
  useMutationDeleteBoard,
} from "../../hooks/use-board";
import { useNavigate } from "@tanstack/react-router";
import { generateSlug } from "random-word-slugs";
import type { Board } from "../../types";
import { authClient } from "@/lib/auth-client";
import { getSession } from "@/lib/auth-utils";
import { queryClient } from "@/lib/query-client";
import {
  PlusIcon,
  FileTextIcon,
  Loader2Icon,
  MoreVerticalIcon,
  ChevronDownIcon,
  LogOutIcon,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { useEffect, useState } from "react";

const DashboardUserButton = () => {
  const [session, setSession] = useState<Awaited<
    ReturnType<typeof getSession>
  > | null>(null);
  const navigate = useNavigate();

  useEffect(() => {
    let isActive = true;

    const loadSession = async () => {
      try {
        const data = await getSession();
        if (isActive) {
          setSession(data);
        }
      } catch (error) {
        console.error("Failed to fetch session:", error);
      }
    };

    loadSession();

    return () => {
      isActive = false;
    };
  }, []);

  if (!session?.user) {
    return null;
  }

  const initials = (session.user.name || session.user.email || "?")
    .split(" ")
    .map((part) => part[0])
    .join("")
    .slice(0, 2)
    .toUpperCase();

  const handleLogout = async () => {
    try {
      await authClient.signOut({
        fetchOptions: {
          onSuccess: () => {
            queryClient.removeQueries({ queryKey: ["session"] });
            navigate({ to: "/login", replace: true });
          },
        },
      });
    } catch (error) {
      console.error("Error signing out:", error);
    }
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="outline"
          className="h-10 gap-3 rounded-full border-border/60 px-3 text-left"
        >
          <div className="flex items-center gap-3">
            {session.user.image ? (
              <img
                src={session.user.image}
                alt={session.user.name}
                className="h-8 w-8 rounded-full object-cover"
              />
            ) : (
              <div className="flex h-8 w-8 items-center justify-center rounded-full bg-muted text-sm font-semibold text-foreground">
                {initials}
              </div>
            )}
          </div>
          <ChevronDownIcon className="size-4 text-muted-foreground" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-64">
        <DropdownMenuLabel>
          <div className="flex flex-col gap-1">
            <span className="font-medium truncate">{session.user.name}</span>
            <span className="text-sm text-muted-foreground truncate">
              {session.user.email}
            </span>
          </div>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          variant="destructive"
          className="cursor-pointer flex items-center justify-between"
          onClick={handleLogout}
        >
          Logout <LogOutIcon className="size-4" />
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
};

export const BoardListView = ({ boards }: { boards: Board[] }) => {
  const createBoard = useMutationCreateBoard();
  const deleteBoard = useMutationDeleteBoard();
  const navigate = useNavigate();

  const handleCreateBoard = () => {
    const boardName = generateSlug();
    createBoard.mutate(boardName, {
      onSuccess: (data) => {
        if (!data?.boardId) {
          return;
        }
        navigate({
          to: "/boards/$boardId",
          params: { boardId: data.boardId.toString() },
        });
      },
      onError: (error) => {
        console.error("Failed to create board:", error);
      },
    });
  };

  const handleBoardClick = (boardId: string) => {
    navigate({
      to: "/boards/$boardId",
      params: { boardId },
    });
  };

  return (
    <div className="flex-1 flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between px-6 py-4 border-b bg-background">
        <div>
          <div className="flex items-center gap-3">
            <img src="/logo.svg" alt="VoicePad Logo" className="h-8 w-8" />
            <h1 className="text-2xl font-semibold text-foreground">Boards</h1>
          </div>
        </div>
        <div className="flex items-center gap-3">
          <Button
            onClick={handleCreateBoard}
            disabled={createBoard.isPending}
            className="gap-2"
          >
            {createBoard.isPending ? (
              <Loader2Icon className="size-4 animate-spin" />
            ) : (
              <PlusIcon className="size-4" />
            )}
            Create board
          </Button>
          <DashboardUserButton />
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-auto">
        {boards.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-full max-w-md mx-auto text-center px-6">
            <div className="rounded-full bg-muted p-6 mb-4">
              <FileTextIcon className="size-12 text-muted-foreground" />
            </div>
            <h2 className="text-xl font-semibold text-foreground mb-2">
              No boards yet
            </h2>
            <p className="text-sm text-muted-foreground mb-6">
              Get started by creating your first board. You can collaborate,
              draw, and share ideas.
            </p>
            <Button
              onClick={handleCreateBoard}
              disabled={createBoard.isPending}
              className="gap-2"
            >
              {createBoard.isPending ? (
                <Loader2Icon className="size-4 animate-spin" />
              ) : (
                <PlusIcon className="size-4" />
              )}
              Create your first board
            </Button>
          </div>
        ) : (
          <div className="max-w-7xl mx-auto px-6 py-6">
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
              {boards.map((board) => (
                <Card
                  key={board.id}
                  onClick={() => handleBoardClick(board.id)}
                  className={cn(
                    "cursor-pointer transition-all hover:shadow-md hover:border-primary/50 group",
                    "hover:-translate-y-0.5"
                  )}
                >
                  <CardHeader className="flex flex-row items-start justify-between space-y-0 pb-2">
                    <CardTitle className="flex items-center gap-2 group-hover:text-primary transition-colors">
                      <FileTextIcon className="size-5 text-muted-foreground group-hover:text-primary transition-colors" />
                      <span className="truncate">{board.name}</span>
                    </CardTitle>
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-8 w-8 p-0"
                          onClick={(event) => {
                            event.stopPropagation();
                          }}
                        >
                          <MoreVerticalIcon className="size-4" />
                          <span className="sr-only">Open menu</span>
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent
                        align="end"
                        onClick={(event) => event.stopPropagation()}
                      >
                        <DropdownMenuItem
                          className="text-destructive"
                          onSelect={(event) => {
                            event.preventDefault();
                            event.stopPropagation();
                            deleteBoard.mutate(board.id);
                          }}
                        >
                          Delete
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </CardHeader>
                  <CardContent>
                    <p className="text-sm text-muted-foreground">
                      Click to open
                    </p>
                  </CardContent>
                </Card>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
