"use client";

import React from "react";
import Link from "next/link";
import { motion } from "motion/react";
import { ArrowUpRight } from "lucide-react";
import { SITE_COPY } from "@/config/copy";
import { SITE_CONFIG } from "@/config/site";

export function HeroSection() {
  return (
    <section className="relative w-full min-h-[88vh] md:min-h-[92vh] flex items-end pb-16 md:pb-24 overflow-hidden bg-panel border-b border-line">
      {/* Background Stage with Slow Ken Burns Scale */}
      <div className="absolute inset-0 overflow-hidden pointer-events-none">
        {/* Subtle architectural hardware studio backdrop */}
        <div
          className="w-full h-full animate-ken-burns bg-cover bg-center opacity-40 mix-blend-multiply"
          style={{
            backgroundImage: `radial-gradient(circle at 65% 35%, #DDD6C6 0%, #EFECE4 45%, #FBF7EF 80%)`,
          }}
        />
        {/* Abstract precision hardware graphic watermark */}
        <svg
          className="absolute right-[-10%] top-[10%] w-[65%] h-[80%] text-line/50 opacity-40 pointer-events-none"
          viewBox="0 0 600 600"
          fill="none"
          xmlns="http://www.w3.org/2000/svg"
        >
          <circle cx="300" cy="300" r="260" stroke="currentColor" strokeWidth="1" strokeDasharray="4 8" />
          <circle cx="300" cy="300" r="180" stroke="currentColor" strokeWidth="1" />
          <rect x="200" y="200" width="200" height="200" rx="4" stroke="currentColor" strokeWidth="1" />
          <path d="M140 300H460M300 140V460" stroke="currentColor" strokeWidth="1" strokeOpacity="0.5" />
        </svg>
      </div>

      <div className="gmc-container relative z-10 w-full">
        <div className="max-w-4xl">
          {/* Micro label */}
          <motion.div
            initial={{ opacity: 0, y: 14 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.6, ease: [0.22, 1, 0.36, 1] }}
          >
            <span className="micro-label text-ink-2 bg-bg/80 backdrop-blur-xs px-3 py-1 rounded-full border border-line">
              {SITE_COPY.hero.label}
            </span>
          </motion.div>

          {/* Staggered Line-by-Line Headline */}
          <h1 className="display-hero text-ink mt-6 font-normal">
            {SITE_COPY.hero.headlineLines.map((line, idx) => (
              <span key={line} className="block overflow-hidden">
                <motion.span
                  initial={{ y: "105%", opacity: 0 }}
                  animate={{ y: 0, opacity: 1 }}
                  transition={{
                    delay: 0.12 * idx + 0.1,
                    duration: 0.85,
                    ease: [0.22, 1, 0.36, 1],
                  }}
                  className="block"
                >
                  {line}
                </motion.span>
              </span>
            ))}
          </h1>

          {/* Subline */}
          <motion.p
            initial={{ opacity: 0, y: 16 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.45, duration: 0.7, ease: [0.22, 1, 0.36, 1] }}
            className="text-[16px] md:text-[18px] text-ink-2 mt-6 max-w-xl leading-relaxed"
          >
            {SITE_COPY.hero.subline}
          </motion.p>

          {/* CTA Buttons */}
          <motion.div
            initial={{ opacity: 0, y: 16 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.6, duration: 0.7, ease: [0.22, 1, 0.36, 1] }}
            className="flex flex-wrap items-center gap-4 mt-8"
          >
            <Link href={SITE_COPY.hero.ctaHref} className="btn-pill-primary">
              {SITE_COPY.hero.ctaLabel}
            </Link>

            <a
              href={SITE_CONFIG.contact.whatsappUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="btn-pill-secondary inline-flex items-center gap-1.5"
            >
              <span>Konsultasi Rakit PC</span>
              <ArrowUpRight className="w-4 h-4" />
            </a>
          </motion.div>
        </div>
      </div>
    </section>
  );
}
