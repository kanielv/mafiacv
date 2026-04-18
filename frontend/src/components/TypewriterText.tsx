import { useEffect, useMemo, useRef, useState } from "react";
import { Text, TextProps } from "@mantine/core";

interface TypewriterTextProps extends Omit<TextProps, "children"> {
  text: string;
  charMs?: number;
  punctuationMs?: number;
  onDone?: () => void;
}

const END_PUNCT = new Set([".", "!", "?", "…"]);
const SHORT_PUNCT = new Set([",", ";", ":"]);

// Worker poll cadence. Workers aren't throttled when the tab is hidden,
// so this fires at full rate regardless of focus — every client progresses
// on the same wall clock and stays in sync.
const POLL_MS = 50;

// Inline worker source: a plain setInterval that posts ticks to main.
// Kept as a string so the build doesn't need a separate worker file.
const WORKER_SRC = `
let id = null;
self.onmessage = (e) => {
  if (e.data?.type === 'start') {
    clearInterval(id);
    id = setInterval(() => self.postMessage('tick'), e.data.pollMs);
  } else if (e.data?.type === 'stop') {
    clearInterval(id);
    id = null;
  }
};
`;

function makeTickWorker(): Worker {
  const blob = new Blob([WORKER_SRC], { type: "application/javascript" });
  const url = URL.createObjectURL(blob);
  const w = new Worker(url);
  // Safe to revoke after construction — the worker has already loaded.
  URL.revokeObjectURL(url);
  return w;
}

export default function TypewriterText({
  text,
  charMs = 40,
  punctuationMs = 180,
  onDone,
  style,
  ...textProps
}: TypewriterTextProps) {
  const [visible, setVisible] = useState(0);
  const onDoneRef = useRef(onDone);
  useEffect(() => { onDoneRef.current = onDone; }, [onDone]);

  const cumulative = useMemo(() => {
    const out = new Array<number>(text.length);
    let t = 0;
    for (let i = 0; i < text.length; i++) {
      const ch = text[i];
      const delay = END_PUNCT.has(ch)
        ? punctuationMs
        : SHORT_PUNCT.has(ch)
        ? Math.round(punctuationMs / 2)
        : charMs;
      t += delay;
      out[i] = t;
    }
    return out;
  }, [text, charMs, punctuationMs]);

  useEffect(() => {
    setVisible(0);
    if (!text) return;

    const start = performance.now();

    const computeVisible = () => {
      const elapsed = performance.now() - start;
      let lo = 0;
      let hi = cumulative.length;
      while (lo < hi) {
        const mid = (lo + hi) >> 1;
        if (cumulative[mid] <= elapsed) lo = mid + 1;
        else hi = mid;
      }
      return lo;
    };

    let cancelled = false;
    let worker: Worker | null = null;

    const onTick = () => {
      if (cancelled) return;
      const v = computeVisible();
      setVisible(v);
      if (v >= text.length) {
        worker?.postMessage({ type: "stop" });
      }
    };

    try {
      worker = makeTickWorker();
      worker.onmessage = onTick;
      worker.postMessage({ type: "start", pollMs: POLL_MS });
    } catch {
      // Fallback: setInterval. Hidden tabs throttle to ~1Hz but the
      // wall-clock math still lands on the correct index when it fires.
      const id = window.setInterval(onTick, POLL_MS);
      const cleanup = () => window.clearInterval(id);
      return () => { cancelled = true; cleanup(); };
    }

    // Prime once so short strings complete on the first paint.
    onTick();

    return () => {
      cancelled = true;
      worker?.postMessage({ type: "stop" });
      worker?.terminate();
    };
  }, [text, cumulative]);

  useEffect(() => {
    if (text.length > 0 && visible >= text.length) {
      onDoneRef.current?.();
    }
  }, [visible, text]);

  return (
    <Text {...textProps} style={{ whiteSpace: "pre-wrap", ...style }}>
      {text.slice(0, visible)}
    </Text>
  );
}
