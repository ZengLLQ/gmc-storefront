import React from "react";
import { cn } from "@/lib/format";

export function GmcWordmark({
  className,
  size = "default",
}: {
  className?: string;
  size?: "default" | "large" | "hero-footer";
}) {
  if (size === "hero-footer") {
    return (
      <div
        className={cn("w-full select-none overflow-hidden leading-none", className)}
        aria-hidden="true"
      >
        <svg
          viewBox="0 0 1200 310"
          className="w-full h-auto block text-ink fill-current"
           preserveAspectRatio="xMidYMid meet"
        >
          <text
            x="50%"
            y="265"
            textAnchor="middle"
            fontFamily="var(--font-sans), Inter, sans-serif"
            fontWeight="500"
            fontSize="335"
            letterSpacing="-18"
          >
            GMC
          </text>
        </svg>
      </div>
    );
  }

  return (
    <span
      className={cn(
        "inline-block font-medium tracking-[-0.055em] text-ink select-none leading-none",
        size === "large" ? "text-3xl md:text-4xl" : "text-2xl md:text-[26px]",
        className
      )}
    >
      GMC
    </span>
  );
}
