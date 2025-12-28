import { createFileRoute } from "@tanstack/react-router";
import { SignUpView } from "@/modules/auth/ui/views/sign-up-view";
import { requireNoAuth } from "@/lib/auth-utils";

export const Route = createFileRoute("/signup")({
  beforeLoad: requireNoAuth,
  component: RouteComponent,
});

function RouteComponent() {
  const appName = "VoicePad";
  return (
    <div
      className="bg-gray-200 flex min-h-svh flex-col items-center justify-center p-6 md:p-10"
      aria-label={`${appName} signup page`}
    >
      <div className="w-full max-w-sm md:max-w-3xl">
        <SignUpView />
      </div>
    </div>
  );
}
