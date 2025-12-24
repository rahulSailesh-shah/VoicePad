import { useNavigate } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { Loader2Icon } from "lucide-react";
import { LiveKitRoom } from "@livekit/components-react";
import { VideoConference } from "../components/video-conference";

const SERVER_URL =
  import.meta.env.VITE_LIVEKIT_SERVER_URL ||
  "wss://conversense-z0ptqzuw.livekit.cloud";

export interface BoardRoomViewProps {
  boardId: string;
}

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
      <div className="flex flex-col items-center justify-center h-screen flex-1">
        <Loader2Icon className="size-12 animate-spin" />
      </div>
    );
  }

  return (
    <div className="h-full w-full bg-amber-600">
      <div>Board room</div>
      <p>Token: {token}</p>
      <LiveKitRoom
        className="h-full w-full"
        serverUrl={SERVER_URL}
        token={token}
        data-lk-theme="default"
        audio={true}
        video={true}
        onDisconnected={() =>
          navigate({
            to: "/boards",
            replace: true,
          })
        }
      >
        {/* <VideoConference meetingId={meetingId} /> */}
      </LiveKitRoom>
    </div>
  );
};
