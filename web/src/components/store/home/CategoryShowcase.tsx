"use client";

import React, { useState } from "react";
import Link from "next/link";
import { AnimatePresence, motion } from "motion/react";
import { ArrowRight } from "lucide-react";
import { cn } from "@/lib/format";

interface CategoryItem {
  id: string;
  slug: string;
  number: string;
  name: string;
  headline: string;
  description: string;
  imageCaption: string;
  bgGradient: string;
}

const CATEGORIES: CategoryItem[] = [
  {
    id: "pc-komponen",
    slug: "komputer-rakitan",
    number: "01",
    name: "PC & Komponen",
    headline: "Fondasi komputasi bertenaga tinggi.",
    description:
      "Prosesor Intel & AMD, kartu grafis NVIDIA RTX, motherboard gaming, modul RAM DDR5, hingga penyimpanan NVMe Gen4/Gen5 pilihan.",
    imageCaption: "CUSTOM RIG · ATX & ITX BUILDS",
    bgGradient: "linear-gradient(135deg, #E6E1D5 0%, #D8D2C4 100%)",
  },
  {
    id: "laptop",
    slug: "laptop",
    number: "02",
    name: "Laptop",
    headline: "Mobilitas tanpa kompromi performa.",
    description:
      "Jajaran ultrabook tipis untuk eksekutif, mobile workstation arsitektur, dan laptop gaming dengan pendingin vapor chamber canggih.",
    imageCaption: "OFFICIAL DISTRIBUTION · LAPTOPS",
    bgGradient: "linear-gradient(135deg, #DDD8CB 0%, #D0CABA 100%)",
  },
  {
    id: "monitor-proyektor",
    slug: "monitor",
    number: "03",
    name: "Monitor & Proyektor",
    headline: "Visual akurat untuk kreasi dan produktivitas.",
    description:
      "Panel IPS 100% sRGB/DCI-P3 terkalibrasi pabrik, layar high-refresh rate untuk esports, serta proyektor resolusi tinggi ruang pertemuan.",
    imageCaption: "CALIBRATED DISPLAYS · HIGH REFRESH",
    bgGradient: "linear-gradient(135deg, #E4DFD3 0%, #D5CEBF 100%)",
  },
  {
    id: "networking-starlink",
    slug: "starlink",
    number: "04",
    name: "Networking & Starlink",
    headline: "Konektivitas satelit dan infrastruktur stabil.",
    description:
      "Kit satelit Starlink Standard resmi, access point enterprise Ubiquiti & TP-Link Omada, switch gigabit, dan kabel jaringan grade industri.",
    imageCaption: "STARLINK SATELLITE & ENTERPRISE MESH",
    bgGradient: "linear-gradient(135deg, #DBD5C7 0%, #CEC7B8 100%)",
  },
  {
    id: "printer-aksesoris",
    slug: "printer",
    number: "05",
    name: "Printer & Aksesoris",
    headline: "Output cetak tajam dan periferal ergonomis.",
    description:
      "Printer EcoTank Brother & Epson untuk volume tinggi, keyboard mekanikal hot-swap, mouse sensor optik presisi, dan docking station.",
    imageCaption: "HIGH CAPACITY PRINT & DESK GEAR",
    bgGradient: "linear-gradient(135deg, #E5E0D4 0%, #D7D0C2 100%)",
  },
  {
    id: "ups-audio",
    slug: "ups",
    number: "06",
    name: "UPS & Audio",
    headline: "Proteksi daya murni dan akustik jernih.",
    description:
      "UPS Pure Sine Wave APC & Eaton pengaman server dari lonjakan listrik, serta monitor audio desktop Edifier untuk audio fidelity prima.",
    imageCaption: "POWER BACKUP & DESKTOP ACOUSTICS",
    bgGradient: "linear-gradient(135deg, #DED9CC 0%, #CFC8B9 100%)",
  },
];

export function CategoryShowcase() {
  const [activeIdx, setActiveIdx] = useState(0);
  const active = CATEGORIES[activeIdx];

  return (
    <section className="py-20 md:py-32 border-b border-line bg-bg">
      <div className="gmc-container">
        {/* Section Header */}
        <div className="flex flex-col md:flex-row md:items-end justify-between gap-6 pb-12 border-b border-line">
          <div>
            <p className="micro-label text-ink-3">KATEGORI UTAMA</p>
            <h2 className="heading-section text-ink mt-2">
              Koleksi perangkat kami.
            </h2>
          </div>
          <p className="text-ink-2 text-sm md:text-base max-w-md">
            Peralatan IT teruji untuk kebutuhan harian, studio kreatif,
            hingga instalasi skala komersial.
          </p>
        </div>

        {/* Desktop Interactive Layout (Synchronized Image & Description Swap) */}
        <div className="hidden lg:grid lg:grid-cols-12 gap-12 pt-12 items-center">
          {/* Left Column: Interactive Categories List */}
          <div className="lg:col-span-7 flex flex-col divide-y divide-line">
            {CATEGORIES.map((cat, idx) => {
              const isActive = idx === activeIdx;
              return (
                <div
                  key={cat.id}
                  onMouseEnter={() => setActiveIdx(idx)}
                  onClick={() => setActiveIdx(idx)}
                  className={cn(
                    "group py-7 cursor-pointer transition-colors duration-200",
                    isActive ? "opacity-100" : "opacity-45 hover:opacity-80"
                  )}
                >
                  <div className="flex items-baseline justify-between">
                    <div className="flex items-baseline gap-4">
                      <span className="micro-label text-ink-3 font-mono">
                        {cat.number}
                      </span>
                      <h3 className="text-3xl xl:text-4xl font-normal tracking-tight text-ink transition-transform duration-300 group-hover:translate-x-1">
                        {cat.name}
                      </h3>
                    </div>
                    <ArrowRight
                      className={cn(
                        "w-5 h-5 transition-all duration-300",
                        isActive
                          ? "opacity-100 translate-x-0"
                          : "opacity-0 -translate-x-2 group-hover:opacity-100 group-hover:translate-x-0"
                      )}
                    />
                  </div>

                  {/* Inline Description on Active */}
                  {isActive && (
                    <motion.div
                      initial={{ opacity: 0, height: 0 }}
                      animate={{ opacity: 1, height: "auto" }}
                      transition={{ duration: 0.35, ease: [0.22, 1, 0.36, 1] }}
                      className="mt-3 pl-9 overflow-hidden"
                    >
                      <p className="text-base text-ink font-medium">
                        {cat.headline}
                      </p>
                      <p className="text-sm text-ink-2 mt-1 max-w-lg leading-relaxed">
                        {cat.description}
                      </p>
                      <Link
                        href={`/shop?category=${cat.slug}`}
                        className="inline-flex items-center gap-1.5 micro-label text-ink hover:underline mt-3"
                      >
                        Lihat koleksi {cat.name} →
                      </Link>
                    </motion.div>
                  )}
                </div>
              );
            })}
          </div>

          {/* Right Column: Large Tall Stage with Crossfading Image */}
          <div className="lg:col-span-5">
            <div className="relative aspect-[4/5] w-full bg-panel overflow-hidden border border-line">
              <AnimatePresence mode="wait">
                <motion.div
                  key={active.id}
                  initial={{ opacity: 0, scale: 0.98 }}
                  animate={{ opacity: 1, scale: 1 }}
                  exit={{ opacity: 0 }}
                  transition={{ duration: 0.5, ease: [0.22, 1, 0.36, 1] }}
                  className="w-full h-full flex flex-col justify-between p-8"
                  style={{ background: active.bgGradient }}
                >
                  <div className="flex justify-between items-start">
                    <span className="micro-label text-ink-3">
                      {active.imageCaption}
                    </span>
                    <span className="micro-label font-mono text-ink-2">
                      {active.number} / 06
                    </span>
                  </div>

                  {/* Artistic Category Icon/Silhouette Stage */}
                  <div className="my-auto flex flex-col items-center justify-center text-center py-10">
                    <div className="w-32 h-32 rounded-full border border-ink/15 flex items-center justify-center text-ink/70">
                      <span className="text-4xl font-light font-mono">
                        {active.number}
                      </span>
                    </div>
                    <p className="text-2xl font-normal text-ink mt-6 tracking-tight">
                      {active.name}
                    </p>
                    <p className="text-xs text-ink-2 max-w-xs mt-2">
                      {active.headline}
                    </p>
                  </div>

                  <div className="pt-4 border-t border-ink/15 flex justify-between items-center">
                    <span className="text-xs text-ink-2">GMC Showroom ITC</span>
                    <Link
                      href={`/shop?category=${active.slug}`}
                      className="btn-pill-primary h-9 px-4 text-[10px]"
                    >
                      Buka Katalog
                    </Link>
                  </div>
                </motion.div>
              </AnimatePresence>
            </div>
          </div>
        </div>

        {/* Mobile View: Vertical Stack of Category Cards */}
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 pt-8 lg:hidden">
          {CATEGORIES.map((cat) => (
            <Link
              key={cat.id}
              href={`/shop?category=${cat.slug}`}
              className="p-6 bg-panel border border-line flex flex-col justify-between h-56 hover:border-ink transition-colors"
            >
              <div className="flex justify-between items-start">
                <span className="micro-label font-mono text-ink-3">
                  {cat.number}
                </span>
                <ArrowRight className="w-4 h-4 text-ink-2" />
              </div>
              <div>
                <h3 className="text-xl font-normal text-ink">{cat.name}</h3>
                <p className="text-xs text-ink-2 mt-1 line-clamp-2">
                  {cat.headline}
                </p>
              </div>
            </Link>
          ))}
        </div>
      </div>
    </section>
  );
}
