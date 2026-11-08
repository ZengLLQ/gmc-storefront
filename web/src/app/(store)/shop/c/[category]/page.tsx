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

export async function generateMetadata({
  params,
}: {
  params: Promise<{ category: string }>;
}): Promise<Metadata> {
  const { category } = await params;
  const formattedTitle = category
    .split("-")
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(" ");

  return {
    title: `${formattedTitle} — Global Mandiri Computer`,
    description: `Koleksi ${formattedTitle} resmi bergaransi distributor di GMC ITC Mega Grosir Surabaya.`,
  };
}

async function getCategoryCatalogData(
  categorySlug: string,
  searchParams: Record<string, string | string[] | undefined>
) {
  const apiBase =
    process.env.INTERNAL_API_URL ||
    process.env.NEXT_PUBLIC_API_URL ||
    "http://127.0.0.1:8080";

  const qs = new URLSearchParams();
  qs.set("category", categorySlug);

  for (const [k, v] of Object.entries(searchParams)) {
    if (typeof v === "string" && v && k !== "category") {
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
    // Fallback to sample data
  }

  return { products, total, categories, brands };
}

export default async function CategoryShopPage({
  params,
  searchParams,
}: {
  params: Promise<{ category: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const { category } = await params;
  const resolvedParams = await searchParams;
  const { products, total, categories, brands } = await getCategoryCatalogData(
    category,
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
        selectedCategorySlug={category}
      />
    </React.Suspense>
  );
}
