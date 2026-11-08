"use client";

import React from "react";
import { SITE_CONFIG } from "@/config/site";

export function BrandMarquee() {
  const brands = SITE_CONFIG.brands;

  return (
    <section className="py-12 border-b border-line overflow-hidden bg-bg">
      <div className="gmc-container mb-4">
        <p className="micro-label text-ink-3">DISTRIBUSI &amp; BRAND PILIHAN</p>
      </div>

      <div className="relative w-full overflow-hidden">
        {/* Infinite marquee ticker container */}
        <div className="animate-marquee flex items-center gap-12 whitespace-nowrap">
          {brands.concat(brands).map((brand, idx) => (
            <div key={`${brand}-${idx}`} className="flex items-center gap-12 shrink-0">
              <span className="text-xl md:text-2xl font-light tracking-tight text-ink-2 hover:text-ink transition-colors cursor-default">
                {brand}
              </span>
              <span className="w-1.5 h-1.5 rounded-full bg-line" aria-hidden="true" />
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
