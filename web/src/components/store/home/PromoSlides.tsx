"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import { AnimatePresence, motion } from "motion/react";
import { ArrowLeft, ArrowRight } from "lucide-react";
import { SITE_CONFIG } from "@/config/site";

interface PromoSlide {
  id: string;
  tag: string;
  headline: string;
  subline: string;
  ctaText: string;
  ctaHref: string;
  isExternal?: boolean;
  bgGradient: string;
}

const SLIDES: PromoSlide[] = [
  {
    id: "pc-rakitan",
    tag: "LAYANAN WORKSHOP · GMC ITC SURABAYA",
    headline: "Rakit PC Impian dengan Presisi Termal.",
    subline:
      "Dukungan pemilihan komponen kompatibel, manajemen kabel tersembunyi, penerapan thermal paste grade industri, dan stress-test 24 jam sebelum serah terima.",
    ctaText: "Konsultasi Rakit PC Sekarang",
    ctaHref: SITE_CONFIG.contact.whatsappUrl,
    isExternal: true,
    bgGradient: "linear-gradient(135deg, #1C1B18 0%, #2A2722 100%)",
  },
  {
    id: "starlink-network",
    tag: "KONEKTIVITAS SATELIT & ENTERPRISE MESH",
    headline: "Internet Cepat di Mana Saja dengan Starlink.",
    subline:
      "Dapatkan Starlink Standard Kit resmi dan access point UniFi untuk jangkauan sinyal tanpa celah di kantor, rumah, kafe, maupun lokasi proyek terpencil.",
    ctaText: "Lihat Perangkat Starlink",
    ctaHref: "/shop?category=starlink",
    isExternal: false,
    bgGradient: "linear-gradient(135deg, #22252A 0%, #15181C 100%)",
  },
];

export function PromoSlides() {
  const [currentIdx, setCurrentIdx] = useState(0);

  useEffect(() => {
    const timer = setInterval(() => {
      setCurrentIdx((prev) => (prev + 1) % SLIDES.length);
    }, 9000);
    return () => clearInterval(timer);
  }, []);

  const slide = SLIDES[currentIdx];

  return (
    <section className="relative w-full border-b border-line overflow-hidden bg-ink text-btn-ink py-20 md:py-32">
      <AnimatePresence mode="wait">
        <motion.div
          key={slide.id}
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.6, ease: [0.22, 1, 0.36, 1] }}
          className="gmc-container relative z-10"
        >
          <div className="max-w-3xl">
            <span className="micro-label text-ink-3 tracking-widest text-[#B5B0A4]">
              {slide.tag}
            </span>

            <h2 className="display-hero text-btn-ink mt-5 font-normal">
              {slide.headline}
            </h2>

            <p className="text-base md:text-lg text-[#CDC8BD] mt-6 max-w-xl leading-relaxed">
              {slide.subline}
            </p>

            <div className="mt-10 flex items-center gap-4">
              {slide.isExternal ? (
                <a
                  href={slide.ctaHref}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center justify-center rounded-full bg-btn-ink text-ink h-12 px-8 micro-label hover:opacity-90 transition-opacity"
                >
                  {slide.ctaText}
                </a>
              ) : (
                <Link
                  href={slide.ctaHref}
                  className="inline-flex items-center justify-center rounded-full bg-btn-ink text-ink h-12 px-8 micro-label hover:opacity-90 transition-opacity"
                >
                  {slide.ctaText}
                </Link>
              )}
            </div>
          </div>
        </motion.div>
      </AnimatePresence>

      {/* Slide Navigation Controls */}
      <div className="gmc-container mt-12 flex items-center justify-between border-t border-line/20 pt-6">
        <div className="flex items-center gap-3">
          {SLIDES.map((s, idx) => (
            <button
              key={s.id}
              type="button"
              onClick={() => setCurrentIdx(idx)}
              aria-label={`Slide ${idx + 1}`}
              className={`h-1.5 transition-all duration-300 rounded-full cursor-pointer ${
                idx === currentIdx ? "w-10 bg-btn-ink" : "w-3 bg-btn-ink/30"
              }`}
            />
          ))}
        </div>

        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() =>
              setCurrentIdx(
                (prev) => (prev - 1 + SLIDES.length) % SLIDES.length
              )
            }
            aria-label="Slide sebelumnya"
            className="w-10 h-10 rounded-full border border-btn-ink/30 flex items-center justify-center text-btn-ink hover:border-btn-ink transition-colors cursor-pointer"
          >
            <ArrowLeft className="w-4 h-4" />
          </button>
          <button
            type="button"
            onClick={() => setCurrentIdx((prev) => (prev + 1) % SLIDES.length)}
            aria-label="Slide berikutnya"
            className="w-10 h-10 rounded-full border border-btn-ink/30 flex items-center justify-center text-btn-ink hover:border-btn-ink transition-colors cursor-pointer"
          >
            <ArrowRight className="w-4 h-4" />
          </button>
        </div>
      </div>
    </section>
  );
}
