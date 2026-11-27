import { useState } from "react";
import { ApiError } from "@/api/client";
import { useAuth } from "@/app/auth";
import { Button } from "@/components/ui/Button";
import { ConfirmDialog } from "@/components/ui/Modal";
import { errorMessage } from "@/components/ui/States";
import { useToast } from "@/components/ui/Toast";
import { userLabel } from "@/lib/format";
import { useClaimTask, useUnclaimTask, type Task } from "./api";
import { claimActions } from "./model";

/** Claim / Release / Take over buttons, with confirmation before taking over someone's task. */
export function ClaimControls({ task, size = "sm" }: { task: Task; size?: "sm" | "md" }) {
  const { user } = useAuth();
  const claim = useClaimTask();
  const unclaim = useUnclaimTask();
  const toast = useToast();
  const [confirmFrom, setConfirmFrom] = useState<string | null>(null);
  const actions = claimActions(task, user);
  if (actions.length === 0) return null;

  const doClaim = async (takeOver: boolean) => {
    try {
      await claim.mutateAsync({ id: task.id, takeOver });
      setConfirmFrom(null);
      toast(takeOver ? "You took over this task" : "Task claimed — it’s now in progress");
    } catch (e) {
      // Someone claimed it a moment ago: offer to take over instead.
      if (e instanceof ApiError && e.code === "already_claimed") setConfirmFrom(e.fields.claimedBy ?? "someone");
      else toast(errorMessage(e), "error");
    }
  };
  const release = async () => {
    try {
      await unclaim.mutateAsync(task.id);
      toast(task.status === "in_progress" ? "Released — the task is open again" : "Claim released");
    } catch (e) {
      toast(errorMessage(e), "error");
    }
  };

  return (
    <>
      {actions.includes("claim") && (
        <Button size={size} variant="primary" loading={claim.isPending} onClick={() => doClaim(false)}>Claim</Button>
      )}
      {actions.includes("takeOver") && (
        <Button size={size} onClick={() => setConfirmFrom(userLabel(task.claimedBy))}>Take over…</Button>
      )}
      {actions.includes("release") && (
        <Button size={size} variant="ghost" loading={unclaim.isPending} onClick={release}>
          {task.claimedBy?.id === user.id ? "Release" : "Release claim"}
        </Button>
      )}
      <ConfirmDialog
        open={confirmFrom !== null}
        title="Take over this task?"
        confirmLabel="Take over"
        danger={false}
        loading={claim.isPending}
        onClose={() => setConfirmFrom(null)}
        onConfirm={() => doClaim(true)}
      >
        <p><span className="font-medium">{confirmFrom}</span> is working on “{task.title}”.</p>
        <p>Taking over assigns it to you. Consider letting them know first, so research isn’t duplicated.</p>
      </ConfirmDialog>
    </>
  );
}
