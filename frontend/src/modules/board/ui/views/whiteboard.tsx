import React, { useState, useRef, useCallback } from "react";
import { Excalidraw } from "@excalidraw/excalidraw";
import "@excalidraw/excalidraw/index.css";
import { convertToExcalidrawElements } from "@excalidraw/excalidraw";
import { Button } from "@/components/ui/button";
import { Diamond, ArrowRight } from "lucide-react";
import { useDebouncedCallback } from "@/lib/use-debounced-callback";

export interface WhiteboardStateChange {
  boardId: string;
  elements: readonly ExcalidrawElement[];
  appState?: unknown;
  files?: unknown;
}

export interface WhiteboardProps {
  boardId: string;
  onStateChange?: (state: WhiteboardStateChange) => void;
}

type ExcalidrawOnChange = Parameters<
  NonNullable<React.ComponentProps<typeof Excalidraw>["onChange"]>
>[0];
type ExcalidrawElement = ExcalidrawOnChange[number];
type ExcalidrawAPI = Parameters<
  NonNullable<React.ComponentProps<typeof Excalidraw>["excalidrawAPI"]>
>[0];

const generateElementId = (type: string) => {
  return `${type}-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
};

export const Whiteboard = ({ boardId, onStateChange }: WhiteboardProps) => {
  const initialElements = convertToExcalidrawElements([], {
    regenerateIds: false,
  });
  const [elements, setElements] =
    useState<readonly ExcalidrawElement[]>(initialElements);
  const excalidrawAPI = useRef<ExcalidrawAPI | null>(null);
  const previousElementsRef =
    useRef<readonly ExcalidrawElement[]>(initialElements);

  const DEBOUNCE_DELAY = 1000;

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
          boardId,
          elements: updatedElements,
          appState,
        });
      }
    },
    DEBOUNCE_DELAY
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

  const addDiamond = useCallback(() => {
    if (!excalidrawAPI.current) return;

    const currentElements = excalidrawAPI.current.getSceneElements();
    const newElement = convertToExcalidrawElements(
      [
        {
          type: "diamond",
          id: generateElementId("diamond"),
          x: 100 + Math.random() * 200,
          y: 100 + Math.random() * 200,
          width: 150,
          height: 100,
          backgroundColor: "#a5d8ff",
          strokeWidth: 2,
        },
      ],
      { regenerateIds: false }
    )[0];

    const updatedElements = [...currentElements, newElement];
    excalidrawAPI.current.updateScene({
      elements: updatedElements,
    });

    excalidrawAPI.current.scrollToContent([newElement]);
  }, []);

  const linkElements = useCallback(() => {
    if (!excalidrawAPI.current) return;

    const currentElements = excalidrawAPI.current.getSceneElements();

    console.log(currentElements);
    const nonDeletedElements = currentElements.filter(
      (el: ExcalidrawElement) => !el.isDeleted && el.type !== "arrow"
    );

    if (nonDeletedElements.length < 2) {
      alert("Need at least 2 elements to create a link");
      return;
    }

    const element1 = nonDeletedElements[nonDeletedElements.length - 2];
    const element2 = nonDeletedElements[nonDeletedElements.length - 1];

    const x1 = element1.x + element1.width / 2;
    const y1 = element1.y + element1.height / 2;
    const x2 = element2.x + element2.width / 2;
    const y2 = element2.y + element2.height / 2;

    const dx = x2 - x1;
    const dy = y2 - y1;

    const arrowSkeleton = {
      type: "arrow" as const,
      x: x1,
      y: y1,
      width: dx,
      height: dy,
      strokeColor: "#1971c2",
      strokeWidth: 2,
      start: {
        id: element1.id,
      },
      end: {
        id: element2.id,
      },
    };

    const [arrowElement] = convertToExcalidrawElements([arrowSkeleton], {
      regenerateIds: false,
    });

    const arrowId = arrowElement.id || generateElementId("arrow");

    const arrowWithBindings: any = {
      ...arrowElement,
      id: arrowId,
      startBinding: {
        elementId: element1.id,
        focus: 0,
        gap: 1,
      },
      endBinding: {
        elementId: element2.id,
        focus: 0,
        gap: 1,
      },
    };

    const updatedElement1: any = {
      ...element1,
      boundElements: [
        ...(element1.boundElements || []),
        {
          id: arrowId,
          type: "arrow",
        },
      ],
    };

    const updatedElement2: any = {
      ...element2,
      boundElements: [
        ...(element2.boundElements || []),
        {
          id: arrowId,
          type: "arrow",
        },
      ],
    };

    // Create updated elements array with modified element1, element2, and new arrow
    const updatedElements = currentElements.map((el) => {
      if (el.id === element1.id) {
        return updatedElement1;
      }
      if (el.id === element2.id) {
        return updatedElement2;
      }
      return el;
    });

    const finalElements = [...updatedElements, arrowWithBindings];
    excalidrawAPI.current.updateScene({
      elements: finalElements,
    });
  }, []);

  return (
    <div className="h-full w-full relative">
      <div className="absolute top-4 left-4 z-10 flex gap-2 flex-wrap">
        <Button
          onClick={addDiamond}
          variant="outline"
          size="sm"
          className="bg-white shadow-md"
        >
          <Diamond className="size-4 mr-2" />
          Add Diamond
        </Button>
        <Button
          onClick={linkElements}
          variant="outline"
          size="sm"
          className="bg-white shadow-md"
        >
          <ArrowRight className="size-4 mr-2" />
          Link Last 2 Elements
        </Button>
      </div>

      <Excalidraw
        excalidrawAPI={handleAPI}
        onChange={handleChange}
        initialData={{
          elements,
          appState: { zenModeEnabled: true, viewBackgroundColor: "#a5d8ff" },
          scrollToContent: true,
        }}
      />
    </div>
  );
};
