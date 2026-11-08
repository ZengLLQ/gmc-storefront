import React from "react";
import type { Metadata } from "next";
import { ShopCatalog } from "@/components/store/shop/ShopCatalog";
import { SAMPLE_PRODUCTS } from "@/lib/data/sampleProducts";
import type {
  ProductCardDTO,
  CategoryDTO,
  BrandDTO,
  ProductListResponseDTO,
} from "@/lib/api/mappers";

export const metadata: Metadata = {
  title: "Shop — Katalog Komputer & Hardware Lengkap",
  description:
    "Jelajahi koleksi prosesor, kartu grafis, motherboard, laptop, monitor, proyektor, networking, dan Starlink di Global Mandiri Computer ITC Surabaya.",
};

async function getCatalogData(
  searchParams: Record<string, string | string[] | undefined>
) {
  const apiBase =
    process.env.INTERNAL_API_URL ||
    process.env.NEXT_PUBLIC_API_URL ||
    "http://127.0.0.1:8080";

  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(searchParams)) {
    if (typeof v === "string" && v) {
      qs.set(k, v);
    }
  }

  let products: ProductCardDTO[] = SAMPLE_PRODUCTS;
  let total = SAMPLE_PRODUCTS.length;
  let categories: CategoryDTO[] = [];
  let brands: BrandDTO[] = [];

  try {
    const [prodRes, catRes, brandRes] = await Promise.all([
      fetch(`${apiBase}/v1/products?${qs.toString()}`, {
        next: { revalidate: 30 },
      }),
      fetch(`${apiBase}/v1/categories`, { next: { revalidate: 300 } }),
      fetch(`${apiBase}/v1/brands`, { next: { revalidate: 300 } }),
    ]);

    if (prodRes.ok) {
      const data: ProductListResponseDTO = await prodRes.json();
      if (data.items && data.items.length > 0) {
        products = data.items;
        total = data.total;
      }
    }

    if (catRes.ok) {
      const catData = await catRes.json();
      categories = catData.items || [];
    }

    if (brandRes.ok) {
      const brandData = await brandRes.json();
      brands = brandData.items || [];
    }
  } catch {
    // Graceful fallback to samples
  }

  return { products, total, categories, brands };
}

export default async function ShopPage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const resolvedParams = await searchParams;
  const { products, total, categories, brands } = await getCatalogData(
    resolvedParams
  );

  return (
    <React.Suspense
      fallback={
        <div className="py-24 text-center min-h-[60vh] flex items-center justify-center">
          <p className="text-sm text-ink-3">Memuat katalog...</p>
        </div>
      }
    >
      <ShopCatalog
        initialProducts={products}
        initialTotal={total}
        categories={categories}
        brands={brands}
      />
    </React.Suspense>
  );
}
