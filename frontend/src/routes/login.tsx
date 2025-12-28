import { createFileRoute } from "@tanstack/react-router";
import { SignInView } from "@/modules/auth/ui/views/sign-in-view";
import { requireNoAuth } from "@/lib/auth-utils";

export const Route = createFileRoute("/login")({
  beforeLoad: requireNoAuth,
  component: RouteComponent,
});

function RouteComponent() {
  return (
    <div className="bg-gray-200 flex min-h-svh flex-col items-center justify-center p-6 md:p-10">
      <div className="w-full max-w-sm md:max-w-3xl">
        <SignInView />
      </div>
    </div>
  );
}
