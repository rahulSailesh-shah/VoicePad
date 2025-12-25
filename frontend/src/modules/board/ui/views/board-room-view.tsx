import { useNavigate } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { Loader2Icon } from "lucide-react";
import { LiveKitRoom, useRoomContext } from "@livekit/components-react";
import { RoomEvent } from "livekit-client";
import { DraggableControlsLayout } from "../components/draggable-controls-layout";
import "@livekit/components-styles";
import { Whiteboard, type WhiteboardStateChange } from "./whiteboard";

const SERVER_URL = "wss://conversense-z0ptqzuw.livekit.cloud";

export interface BoardRoomViewProps {
  boardId: string;
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

export const BoardRoomView = ({ boardId }: BoardRoomViewProps) => {
  const navigate = useNavigate();
  const [token, setToken] = useState<string | null>(null);

  useEffect(() => {
    const storedToken = sessionStorage.getItem(`room-token`);
    if (!storedToken) {
      navigate({
        to: "/boards",
        replace: true,
      });
      return;
    }
    setToken(storedToken);
  }, [navigate]);

  if (!token) {
    return (
      <div className="flex flex-col items-center justify-center h-screen w-screen bg-white">
        <Loader2Icon className="size-12 animate-spin" />
      </div>
    );
  }

  const handleStateChange = (state: WhiteboardStateChange) => {
    console.log("state", state);
  };

  return (
    <div className="h-screen w-screen relative bg-white">
      <Whiteboard boardId={boardId} onStateChange={handleStateChange} />
      {/* 
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
        <div className="h-full w-full relative">
          <div
            className="absolute inset-0"
            style={{ width: "100%", height: "100%" }}
          >
            <Whiteboard boardId={boardId} />
          </div>
          <DraggableControlsLayout meetingId={boardId} />
        </div>
      </LiveKitRoom> */}
    </div>
  );
};
