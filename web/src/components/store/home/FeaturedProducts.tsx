"use client";

import React, { useMemo, useState } from "react";
import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { ProductCard } from "../ProductCard";
import type { ProductCardDTO } from "@/lib/api/mappers";

const CHIP_FILTERS = [
  { id: "all", label: "Semua Produk", slug: "" },
  { id: "pc", label: "PC & Komponen", slug: "komputer-rakitan" },
  { id: "laptop", label: "Laptop", slug: "laptop" },
  { id: "monitor", label: "Monitor", slug: "monitor" },
  { id: "networking", label: "Networking & Starlink", slug: "starlink" },
  { id: "audio", label: "Audio & Aksesoris", slug: "speaker" },
];

export function FeaturedProducts({
  initialProducts = [],
}: {
  initialProducts?: ProductCardDTO[];
}) {
  const [activeFilter, setActiveFilter] = useState("all");

  const filteredProducts = useMemo(() => {
    if (activeFilter === "all") return initialProducts.slice(0, 12);

    return initialProducts
      .filter((p) => {
        if (activeFilter === "pc") {
          return (
            p.category?.slug.includes("komputer") ||
            p.category?.slug.includes("cpu") ||
            p.category?.slug.includes("gpu") ||
            p.category?.slug.includes("motherboard") ||
            p.category?.slug.includes("ram") ||
            p.category?.slug.includes("psu")
          );
        }
        if (activeFilter === "laptop") {
          return p.category?.slug.includes("laptop");
        }
        if (activeFilter === "monitor") {
          return (
            p.category?.slug.includes("monitor") ||
            p.category?.slug.includes("proyektor")
          );
        }
        if (activeFilter === "networking") {
          return (
            p.category?.slug.includes("starlink") ||
            p.category?.slug.includes("networking")
          );
        }
        if (activeFilter === "audio") {
          return (
            p.category?.slug.includes("speaker") ||
            p.category?.slug.includes("keyboard") ||
            p.category?.slug.includes("mouse") ||
            p.category?.slug.includes("ups")
          );
        }
        return true;
      })
      .slice(0, 12);
  }, [activeFilter, initialProducts]);

  return (
    <section className="py-20 md:py-32 border-b border-line bg-bg">
      <div className="gmc-container">
        {/* Header and Filter Chips */}
        <div className="flex flex-col md:flex-row md:items-end justify-between gap-6 pb-8 border-b border-line">
          <div>
            <p className="micro-label text-ink-3">KURASI TERBAIK</p>
            <h2 className="heading-section text-ink mt-2">Produk Pilihan.</h2>
          </div>

          <Link
            href="/shop"
            className="inline-flex items-center gap-1.5 micro-label text-ink hover:underline group"
          >
            <span>Lihat Semua Produk</span>
            <ArrowRight className="w-4 h-4 transition-transform group-hover:translate-x-1" />
          </Link>
        </div>

        {/* Filter Chips Bar */}
        <div className="flex items-center gap-2.5 overflow-x-auto py-6 no-scrollbar">
          {CHIP_FILTERS.map((f) => {
            const isActive = activeFilter === f.id;
            return (
              <button
                key={f.id}
                type="button"
                onClick={() => setActiveFilter(f.id)}
                className={`chip-pill shrink-0 ${
                  isActive ? "chip-pill-active" : ""
                }`}
              >
                {f.label}
              </button>
            );
          })}
        </div>

        {/* 4-column desktop / 2-column mobile Product Grid */}
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-x-6 gap-y-12 pt-6">
          {filteredProducts.map((p) => (
            <ProductCard key={p.id} product={p} />
          ))}
        </div>

        {filteredProducts.length === 0 && (
          <div className="text-center py-20 text-ink-3 text-sm">
            Belum ada produk untuk kategori ini.
          </div>
        )}

        {/* Bottom CTA */}
        <div className="mt-16 text-center">
          <Link href="/shop" className="btn-pill-secondary">
            Jelajahi Seluruh Katalog Toko
          </Link>
        </div>
      </div>
    </section>
  );
}
