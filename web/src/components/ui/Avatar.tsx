import { useState } from "react";
import { mediaFileUrl } from "@/api/client";

const tones: Record<string, string> = {
  M: "bg-sky-100 text-sky-800 dark:bg-sky-900/60 dark:text-sky-200",
  F: "bg-rose-100 text-rose-800 dark:bg-rose-900/60 dark:text-rose-200",
  X: "bg-violet-100 text-violet-800 dark:bg-violet-900/60 dark:text-violet-200",
  U: "bg-stone-200 text-stone-700 dark:bg-stone-700 dark:text-stone-200",
};

export function Avatar({ name, sex = "U", mediaId, size = 40 }: { name: string; sex?: string; mediaId?: number | null; size?: number }) {
  const [broken, setBroken] = useState(false);
  const initials = name.split(/\s+/).filter(Boolean).slice(0, 2).map((s) => s[0]?.toUpperCase()).join("") || "?";
  const style = { width: size, height: size, fontSize: size * 0.38 };
  if (mediaId && !broken) {
    return <img src={mediaFileUrl(mediaId)} alt="" style={style} onError={() => setBroken(true)} className="shrink-0 rounded-full object-cover ring-1 ring-stone-200 dark:ring-stone-700" />;
  }
  return (
    <div style={style} aria-hidden="true" className={`flex shrink-0 items-center justify-center rounded-full font-semibold ${tones[sex] ?? tones.U}`}>
      {initials}
    </div>
  );
}
