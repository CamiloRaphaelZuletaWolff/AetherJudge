import { Spinner } from "@/components/ui/spinner";

export default function RoomLoading() {
  return (
    <main className="flex min-h-screen items-center justify-center text-muted">
      <Spinner />
    </main>
  );
}
