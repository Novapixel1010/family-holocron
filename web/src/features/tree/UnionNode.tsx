import { memo } from "react";
import { Handle, Position, type NodeProps } from "@xyflow/react";
import { titleCase } from "@/lib/format";
import { UNION_SIZE, type UnionFlowNode } from "./graph";

const hiddenHandle = "!h-px !w-px !min-h-0 !min-w-0 !border-0 !bg-transparent !opacity-0";

/** Junction where a couple's (or parent set's) lines meet before reaching their children. */
export const UnionNode = memo(function UnionNode({ data }: NodeProps<UnionFlowNode>) {
  const ended = data.subtype === "divorced" || data.subtype === "separated";
  const label = data.couple ? titleCase(data.subtype || "partners") : "Shared parents";
  return (
    <div title={label} aria-label={label} style={{ width: UNION_SIZE, height: UNION_SIZE }}
      className={`rounded-full ${data.couple ? "bg-rose-400 dark:bg-rose-500" : "bg-stone-400 dark:bg-stone-500"} ${ended ? "!bg-transparent border-2 border-dashed border-rose-400" : "ring-2 ring-white dark:ring-stone-950"}`}>
      <Handle type="target" position={Position.Top} id="top" isConnectable={false} className={hiddenHandle} />
      <Handle type="source" position={Position.Bottom} id="bottom" isConnectable={false} className={hiddenHandle} />
    </div>
  );
});
