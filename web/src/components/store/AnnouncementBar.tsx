"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import { AnimatePresence, motion } from "motion/react";
import { SITE_COPY } from "@/config/copy";

export interface AnnouncementItem {
  id?: string;
  text: string;
  linkUrl?: string | null;
}

export function AnnouncementBar({
  items,
}: {
  items?: AnnouncementItem[];
}) {
  const list: AnnouncementItem[] =
    items && items.length > 0
      ? items
      : SITE_COPY.announcements.map((text, idx) => ({
          id: String(idx),
          text,
          linkUrl: idx === 0 ? "/contact" : idx === 1 ? "/shop" : "/contact#garansi",
        }));

  const [index, setIndex] = useState(0);

  useEffect(() => {
    if (list.length <= 1) return;
    const timer = setInterval(() => {
      setIndex((prev) => (prev + 1) % list.length);
    }, 5000);
    return () => clearInterval(timer);
  }, [list.length]);

  const current = list[index] || list[0];
  if (!current) return null;

  return (
    <div className="w-full bg-panel border-b border-line h-9 flex items-center justify-center px-4 overflow-hidden">
      <AnimatePresence mode="wait">
        <motion.div
          key={current.id || current.text}
          initial={{ opacity: 0, y: 6 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: -6 }}
          transition={{ duration: 0.4, ease: [0.22, 1, 0.36, 1] }}
          className="text-center"
        >
          {current.linkUrl ? (
            <Link
              href={current.linkUrl}
              className="micro-label text-ink-2 hover:text-ink transition-colors"
            >
              {current.text}
            </Link>
          ) : (
            <span className="micro-label text-ink-2">{current.text}</span>
          )}
        </motion.div>
      </AnimatePresence>
    </div>
  );
}
