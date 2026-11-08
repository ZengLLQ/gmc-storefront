import React from "react";
import type { Metadata } from "next";
import { HeroSection } from "@/components/store/home/HeroSection";
import { CategoryShowcase } from "@/components/store/home/CategoryShowcase";
import { FeaturedProducts } from "@/components/store/home/FeaturedProducts";
import { PromoSlides } from "@/components/store/home/PromoSlides";
import { WhyGmc } from "@/components/store/home/WhyGmc";
import { BrandMarquee } from "@/components/store/home/BrandMarquee";
import { VisitUs } from "@/components/store/home/VisitUs";
import { SAMPLE_PRODUCTS } from "@/lib/data/sampleProducts";
import type { ProductCardDTO } from "@/lib/api/mappers";

export const metadata: Metadata = {
  title: "Global Mandiri Computer — ITC Mega Grosir Surabaya",
  description:
    "Toko komputer & hardware IT terpercaya di ITC Mega Grosir Surabaya. Sedia PC rakitan, laptop, kartu grafis, monitor, printer, networking, dan Starlink resmi.",
};

async function getFeaturedProducts(): Promise<ProductCardDTO[]> {
  try {
    const apiBase =
      process.env.INTERNAL_API_URL ||
      process.env.NEXT_PUBLIC_API_URL ||
      "http://127.0.0.1:8080";
    const res = await fetch(`${apiBase}/v1/products?featured=true&per_page=12`, {
      next: { revalidate: 60 },
      headers: { Accept: "application/json" },
    });
    if (!res.ok) {
      return SAMPLE_PRODUCTS;
    }
    const data = await res.json();
    return data.items && data.items.length > 0 ? data.items : SAMPLE_PRODUCTS;
  } catch {
    return SAMPLE_PRODUCTS;
  }
}

export default async function HomePage() {
  const products = await getFeaturedProducts();

  return (
    <div className="flex flex-col">
      <HeroSection />
      <CategoryShowcase />
      <FeaturedProducts initialProducts={products} />
      <PromoSlides />
      <WhyGmc />
      <BrandMarquee />
      <VisitUs />
    </div>
  );
}
