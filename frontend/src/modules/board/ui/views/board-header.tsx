import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb";
import { Input } from "@/components/ui/input";
import { useMutationUpdateBoard } from "../../hooks/use-board";

import { Link } from "@tanstack/react-router";
import { useEffect, useRef, useState } from "react";

export const BoardNameInput = ({
  boardId,
  boardName,
}: {
  boardId: string;
  boardName: string;
}) => {
  const updateBoard = useMutationUpdateBoard();
  const [isEditing, setIsEditing] = useState(false);
  const [name, setName] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (boardName) {
      setName(boardName);
    }
  }, [boardName]);

  useEffect(() => {
    if (isEditing && inputRef.current) {
      inputRef.current.focus();
      inputRef.current.select();
    }
  }, [isEditing]);

  const handleSave = async () => {
    if (name === boardName) {
      setIsEditing(false);
      return;
    }

    try {
      if (inputRef.current) {
        updateBoard.mutateAsync({
          id: boardId,
          req: {
            name: inputRef.current?.value || "",
          },
        });
        setIsEditing(false);
      }
    } catch {
      setName(boardName);
    } finally {
      setIsEditing(false);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") {
      handleSave();
    } else if (e.key === "Escape") {
      setName(boardName);
      setIsEditing(false);
    }
  };

  if (isEditing) {
    return (
      <BreadcrumbItem className="cursor-pointer hover:text-foreground transition-colors">
        <Input
          ref={inputRef}
          value={name}
          onChange={(e) => setName(e.target.value)}
          onBlur={handleSave}
          onKeyDown={handleKeyDown}
          className="w-full text-foreground bg-transparent border-none outline-none"
          disabled={updateBoard.isPending}
        />
      </BreadcrumbItem>
    );
  }

  return (
    <BreadcrumbItem
      className="cursor-pointer hover:text-foreground transition-colors"
      onClick={() => setIsEditing(true)}
    >
      {boardName}
    </BreadcrumbItem>
  );
};

export const BoardBreadcrumb = ({
  boardId,
  boardName,
}: {
  boardId: string;
  boardName: string;
}) => {
  return (
    <Breadcrumb>
      <BreadcrumbList>
        <BreadcrumbItem>
          <BreadcrumbLink asChild>
            <Link to={`/boards`}>Boards</Link>
          </BreadcrumbLink>
        </BreadcrumbItem>
        <BreadcrumbSeparator />
        <BoardNameInput boardId={boardId} boardName={boardName} />
      </BreadcrumbList>
    </Breadcrumb>
  );
};

const BoardHeader = ({
  boardId,
  boardName,
}: {
  boardId: string;
  boardName: string;
}) => {
  return (
    <div className="flex h-14 shrink-0 items-center gap-2 border-b px-4 bg-background sticky top-0 z-50 font-sans">
      <div className="flex flex-row items-center justify-between gap-x-4 w-full">
        <div className="flex items-center gap-2">
          <img src="/logo.svg" alt="Draw Logo" className="h-8 w-8" />
          <BoardBreadcrumb boardId={boardId} boardName={boardName} />
        </div>
      </div>
    </div>
  );
};

export default BoardHeader;
