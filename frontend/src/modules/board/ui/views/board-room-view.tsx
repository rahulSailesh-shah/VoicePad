import { useNavigate } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { RoomEvent } from "livekit-client";
import "@livekit/components-styles";
import {
  Whiteboard,
  type WhiteboardStateChange,
  convertFromExcalidrawElements,
} from "./whiteboard";
import type { Board } from "../../types";
import { DraggableControlsLayout } from "../components/draggable-controls-layout";
import { LiveKitRoom, useRoomContext } from "@livekit/components-react";
import { useMutationUpdateBoard } from "../../hooks/use-board";

const SERVER_URL = "wss://conversense-z0ptqzuw.livekit.cloud";

export interface BoardRoomViewProps {
  board: Board;
  token: string;
}

const MuteOnJoin = () => {
  const room = useRoomContext();

  useEffect(() => {
    if (!room) return;

    const configureTracks = () => {
      if (room.localParticipant) {
        room.localParticipant.setMicrophoneEnabled(false);
        room.localParticipant.setCameraEnabled(false);
      }
    };

    if (room.state === "connected") {
      configureTracks();
    }

    room.on(RoomEvent.Connected, configureTracks);

    return () => {
      room.off(RoomEvent.Connected, configureTracks);
    };
  }, [room]);

  return null;
};

export const BoardRoomView = ({ board, token }: BoardRoomViewProps) => {
  const navigate = useNavigate();
  const { mutate: updateBoardMutation } = useMutationUpdateBoard();
  const [llmResponse, setLlmResponse] = useState<any>(null);

  if (!token) {
    return (
      <div className="flex flex-col items-center justify-center h-screen w-screen bg-white">
        Something went wrong. Please try again.
      </div>
    );
  }

  const handleStateChange = (state: WhiteboardStateChange) => {
    console.log(state.elements);
    const commands = convertFromExcalidrawElements(state.elements);
    updateBoardMutation({
      id: board.id.toString(),
      req: {
        elements: commands,
      },
    });
  };

  const onCanvasUpdate = (response: any) => {
    console.log("Canvas update:", response);
    setLlmResponse(response);
  };

  return (
    <div className="h-screen w-screen relative bg-white overflow-hidden">
      <LiveKitRoom
        className="h-full w-full relative"
        serverUrl={SERVER_URL}
        token={token}
        data-lk-theme="default"
        audio={true}
        video={false}
        onDisconnected={() =>
          navigate({
            to: "/boards",
            replace: true,
          })
        }
      >
        <MuteOnJoin />
        <div className="h-full w-full relative overflow-hidden">
          <div
            className="absolute inset-0"
            style={{ width: "100%", height: "100%" }}
          >
            <Whiteboard
              board={board}
              onStateChange={handleStateChange}
              llmResponse={llmResponse}
              onLlmResponseProcessed={() => setLlmResponse(null)}
            />
          </div>
          <DraggableControlsLayout onCanvasUpdate={onCanvasUpdate} />
        </div>
      </LiveKitRoom>
    </div>
  );
};
