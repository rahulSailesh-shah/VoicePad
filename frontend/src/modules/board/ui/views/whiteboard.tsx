import React, {
  useState,
  useRef,
  useCallback,
  useEffect,
  useMemo,
} from "react";
import { Excalidraw } from "@excalidraw/excalidraw";
import "@excalidraw/excalidraw/index.css";
import { convertToExcalidrawElements } from "@excalidraw/excalidraw";
import { useDebouncedCallback } from "@/lib/use-debounced-callback";
import BoardHeader from "./board-header";
import type { Board } from "../../types";
import type { ExcalidrawElementSkeleton } from "@excalidraw/excalidraw/data/transform";
import type { OrderedExcalidrawElement } from "@excalidraw/excalidraw/element/types";

export interface WhiteboardStateChange {
  elements: readonly ExcalidrawElement[];
  appState?: unknown;
  files?: unknown;
}

export interface WhiteboardProps {
  board: Board;
  onStateChange?: (state: WhiteboardStateChange) => void;
  llmResponse?: any;
  onLlmResponseProcessed?: () => void;
}

type ExcalidrawOnChange = Parameters<
  NonNullable<React.ComponentProps<typeof Excalidraw>["onChange"]>
>[0];
type ExcalidrawElement = ExcalidrawOnChange[number];
type ExcalidrawAPI = Parameters<
  NonNullable<React.ComponentProps<typeof Excalidraw>["excalidrawAPI"]>
>[0];

export function convertFromExcalidrawElements(
  elements: readonly ExcalidrawElement[]
): Record<string, any>[] {
  const activeElements = elements.filter((el) => !el.isDeleted);
  return activeElements.map((element) => JSON.parse(JSON.stringify(element)));
}

export const Whiteboard = ({
  board,
  onStateChange,
  llmResponse,
  onLlmResponseProcessed,
}: WhiteboardProps) => {
  const initialElements: OrderedExcalidrawElement[] = useMemo(() => {
    const boardElements = Array.isArray(board.elements)
      ? (board.elements as ExcalidrawElementSkeleton[])
      : [];

    try {
      return convertToExcalidrawElements(boardElements, {
        regenerateIds: false,
      });
    } catch (error) {
      console.error("Failed to convert board elements", {
        error,
        boardId: board.id,
        elements: board.elements,
      });
      return [];
    }
  }, [board.elements, board.id]);

  const excalidrawAPI = useRef<ExcalidrawAPI | null>(null);
  const previousElementsRef =
    useRef<readonly ExcalidrawElement[]>(initialElements);

  const [elements, setElements] =
    useState<readonly ExcalidrawElement[]>(initialElements);

  useEffect(() => {
    setElements(initialElements);
    previousElementsRef.current = initialElements;
  }, [initialElements]);

  const elementsHaveChanged = useCallback(
    (
      prevElements: readonly ExcalidrawElement[],
      newElements: readonly ExcalidrawElement[]
    ): boolean => {
      if (prevElements.length !== newElements.length) {
        return true;
      }

      const prevMap = new Map(prevElements.map((el) => [el.id, el]));
      const newMap = new Map(newElements.map((el) => [el.id, el]));

      if (prevMap.size !== newMap.size) {
        return true;
      }

      let hasChanges = false;
      for (const [id, newEl] of newMap) {
        const prevEl = prevMap.get(id);
        if (!prevEl) {
          return true;
        }

        if (prevEl.version !== newEl.version) {
          hasChanges = true;
          break;
        }

        if (prevEl.isDeleted !== newEl.isDeleted) {
          hasChanges = true;
          break;
        }

        const positionChanged =
          Math.abs(prevEl.x - newEl.x) > 0.01 ||
          Math.abs(prevEl.y - newEl.y) > 0.01;
        const sizeChanged =
          Math.abs(prevEl.width - newEl.width) > 0.01 ||
          Math.abs(prevEl.height - newEl.height) > 0.01;

        if (positionChanged || sizeChanged) {
          hasChanges = true;
          break;
        }
      }

      return hasChanges;
    },
    []
  );

  const notifyStateChange = useDebouncedCallback(
    (updatedElements: readonly ExcalidrawElement[], appState?: unknown) => {
      if (onStateChange) {
        onStateChange({
          elements: updatedElements,
          appState,
        });
      }
    }
  );

  const handleChange = useCallback(
    (updatedElements: readonly ExcalidrawElement[], appState: unknown) => {
      const elementsChanged = elementsHaveChanged(
        previousElementsRef.current,
        updatedElements
      );
      setElements(updatedElements);
      previousElementsRef.current = updatedElements.map((el) => ({ ...el }));

      if (elementsChanged) {
        notifyStateChange(updatedElements, appState);
      }
    },
    [notifyStateChange, elementsHaveChanged]
  );

  const handleAPI = useCallback((api: ExcalidrawAPI) => {
    excalidrawAPI.current = api;
  }, []);

  useEffect(() => {
    if (!llmResponse || !excalidrawAPI.current) return;

    try {
      let response: {
        action: "add" | "update" | "delete";
        elements?: ExcalidrawElementSkeleton[];
        delete_ids?: string[];
      };

      if (typeof llmResponse === "string") {
        response = JSON.parse(llmResponse);
      } else {
        response = llmResponse;
      }

      const currentElements = excalidrawAPI.current.getSceneElements();

      if (response.action === "delete" && response.delete_ids) {
        const updatedElements = currentElements.map((el) => {
          if (response.delete_ids?.includes(el.id)) {
            return { ...el, isDeleted: true };
          }
          return el;
        });
        excalidrawAPI.current.updateScene({ elements: updatedElements });
        setElements(updatedElements);
      } else if (
        (response.action === "add" || response.action === "update") &&
        response.elements
      ) {
        const newElements = convertToExcalidrawElements(response.elements, {
          regenerateIds: false,
        });

        if (response.action === "add") {
          const updatedElements = [...currentElements, ...newElements];
          excalidrawAPI.current.updateScene({ elements: updatedElements });
          setElements(updatedElements);
        } else if (response.action === "update") {
          const elementMap = new Map(currentElements.map((el) => [el.id, el]));
          newElements.forEach((newEl) => {
            if (elementMap.has(newEl.id)) {
              const existingEl = elementMap.get(newEl.id)!;
              elementMap.set(newEl.id, { ...existingEl, ...newEl });
            }
          });

          const updatedElements = Array.from(elementMap.values());
          excalidrawAPI.current.updateScene({ elements: updatedElements });
          setElements(updatedElements);
        }
      }

      if (onLlmResponseProcessed) {
        onLlmResponseProcessed();
      }
    } catch (error) {
      console.error("Error processing LLM response:", error);
      if (onLlmResponseProcessed) {
        onLlmResponseProcessed();
      }
    }
  }, [llmResponse, onLlmResponseProcessed]);

  return (
    <div className="h-full w-full relative flex flex-col">
      <BoardHeader boardId={board.id} boardName={board.name} />
      <div
        className="flex-1 relative"
        style={{ height: "calc(100% - 3.5rem)" }}
      >
        <div className="excalidraw-wrapper" style={{ height: "100%" }}>
          <Excalidraw
            excalidrawAPI={handleAPI}
            onChange={handleChange}
            initialData={{ elements, appState: { theme: "light" } }}
          />
        </div>
      </div>
    </div>
  );
};
