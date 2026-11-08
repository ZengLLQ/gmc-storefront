"use client";

import React, { useState } from "react";
import { AnimatePresence, motion } from "motion/react";
import { SITE_COPY } from "@/config/copy";
import { cn } from "@/lib/format";

export function WhyGmc() {
  const [activeTab, setActiveTab] = useState(0);
  const currentPillar = SITE_COPY.whyGmc.pillars[activeTab];

  return (
    <section className="py-20 md:py-32 border-b border-line bg-bg">
      <div className="gmc-container">
        {/* Section Heading */}
        <div className="max-w-2xl pb-12 border-b border-line">
          <p className="micro-label text-ink-3">{SITE_COPY.whyGmc.label}</p>
          <h2 className="heading-section text-ink mt-2">
            {SITE_COPY.whyGmc.heading}
          </h2>
        </div>

        {/* Tab Selection Bar */}
        <div className="flex items-center gap-2 overflow-x-auto py-6 border-b border-line no-scrollbar">
          {SITE_COPY.whyGmc.pillars.map((pillar, idx) => {
            const isActive = idx === activeTab;
            return (
              <button
                key={pillar.id}
                type="button"
                onClick={() => setActiveTab(idx)}
                className={cn(
                  "px-5 py-3 rounded-full micro-label whitespace-nowrap transition-all duration-300 cursor-pointer",
                  isActive
                    ? "bg-ink text-bg"
                    : "bg-transparent text-ink-2 border border-line hover:border-ink hover:text-ink"
                )}
              >
                {pillar.tabLabel}
              </button>
            );
          })}
        </div>

        {/* Synced Content & Architectural Stage */}
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-12 pt-12 items-center">
          {/* Left Text */}
          <div className="lg:col-span-6">
            <AnimatePresence mode="wait">
              <motion.div
                key={currentPillar.id}
                initial={{ opacity: 0, y: 16 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: -16 }}
                transition={{ duration: 0.45, ease: [0.22, 1, 0.36, 1] }}
                className="space-y-6"
              >
                <span className="micro-label text-ink-3 font-mono">
                  PILAR 0{activeTab + 1} / 05
                </span>
                <h3 className="heading-sub text-ink font-normal">
                  {currentPillar.title}
                </h3>
                <p className="text-base md:text-lg text-ink-2 leading-relaxed">
                  {currentPillar.body}
                </p>
                <div className="pt-4 border-t border-line">
                  <span className="micro-label text-ink-3">
                    {currentPillar.caption}
                  </span>
                </div>
              </motion.div>
            </AnimatePresence>
          </div>

          {/* Right Synced Image Stage */}
          <div className="lg:col-span-6">
            <div className="aspect-[4/3] w-full bg-panel border border-line overflow-hidden relative p-8 flex flex-col justify-between">
              <AnimatePresence mode="wait">
                <motion.div
                  key={currentPillar.id}
                  initial={{ opacity: 0, scale: 0.98 }}
                  animate={{ opacity: 1, scale: 1 }}
                  exit={{ opacity: 0 }}
                  transition={{ duration: 0.5, ease: [0.22, 1, 0.36, 1] }}
                  className="w-full h-full flex flex-col justify-between"
                >
                  <div className="flex justify-between items-start">
                    <span className="micro-label text-ink-3">GMC STANDAR</span>
                    <span className="text-3xl font-light font-mono text-ink-3">
                      0{activeTab + 1}
                    </span>
                  </div>

                  <div className="my-auto text-center py-8">
                    <div className="inline-block p-6 rounded-full border border-ink/20 mb-4">
                      <span className="micro-label text-ink tracking-widest">
                        SURABAYA
                      </span>
                    </div>
                    <p className="text-2xl font-light text-ink tracking-tight">
                      {currentPillar.tabLabel}
                    </p>
                    <p className="text-xs text-ink-2 mt-2 max-w-xs mx-auto">
                      ITC Mega Grosir Lantai 2 Blok B/7
                    </p>
                  </div>

                  <div className="flex justify-between items-center text-xs text-ink-3 border-t border-line/60 pt-3">
                    <span>Komitmen Mutu &amp; Distribusi Resmi</span>
                    <span className="font-mono">VERIFIED</span>
                  </div>
                </motion.div>
              </AnimatePresence>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
