import { requireAuth } from "@/lib/auth-utils";
import { createFileRoute, redirect } from "@tanstack/react-router";

export const Route = createFileRoute("/")({
  beforeLoad: async () => {
    await requireAuth();
    throw redirect({ to: "/boards" });
  },
  component: RouteComponent,
});

function RouteComponent() {
  return <></>;
}
