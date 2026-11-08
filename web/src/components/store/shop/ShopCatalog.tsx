"use client";

import React, { useEffect, useState, useTransition } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Search, SlidersHorizontal, X, ChevronDown, Check } from "lucide-react";
import { ProductCard } from "@/components/store/ProductCard";
import type { ProductCardDTO, CategoryDTO, BrandDTO } from "@/lib/api/mappers";
import { formatIDR, cn } from "@/lib/format";
import { SAMPLE_PRODUCTS } from "@/lib/data/sampleProducts";

interface ShopCatalogProps {
  initialProducts: ProductCardDTO[];
  initialTotal: number;
  categories: CategoryDTO[];
  brands: BrandDTO[];
  selectedCategorySlug?: string;
}

const SORT_OPTIONS = [
  { value: "newest", label: "Terbaru" },
  { value: "price_asc", label: "Harga: Rendah ke Tinggi" },
  { value: "price_desc", label: "Harga: Tinggi ke Rendah" },
  { value: "name_asc", label: "Nama: A — Z" },
];

export function ShopCatalog({
  initialProducts,
  initialTotal,
  categories,
  brands,
  selectedCategorySlug,
}: ShopCatalogProps) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [, startTransition] = useTransition();

  // Local state initialized from searchParams
  const [searchQuery, setSearchQuery] = useState(searchParams.get("q") || "");
  const [activeCategory, setActiveCategory] = useState(
    selectedCategorySlug || searchParams.get("category") || ""
  );
  const [activeBrand, setActiveBrand] = useState(searchParams.get("brand") || "");
  const [minPrice, setMinPrice] = useState(searchParams.get("min_price") || "");
  const [maxPrice, setMaxPrice] = useState(searchParams.get("max_price") || "");
  const [inStockOnly, setInStockOnly] = useState(
    searchParams.get("in_stock") === "true"
  );
  const [sort, setSort] = useState(searchParams.get("sort") || "newest");
  const [mobileFilterOpen, setMobileFilterOpen] = useState(false);

  // Sync state if URL changes externally
  useEffect(() => {
    setSearchQuery(searchParams.get("q") || "");
    setActiveCategory(
      selectedCategorySlug || searchParams.get("category") || ""
    );
    setActiveBrand(searchParams.get("brand") || "");
    setMinPrice(searchParams.get("min_price") || "");
    setMaxPrice(searchParams.get("max_price") || "");
    setInStockOnly(searchParams.get("in_stock") === "true");
    setSort(searchParams.get("sort") || "newest");
  }, [searchParams, selectedCategorySlug]);

  const applyFilters = (overrides?: Record<string, string | null>) => {
    const params = new URLSearchParams(searchParams.toString());

    const nextState = {
      q: searchQuery.trim(),
      category: activeCategory,
      brand: activeBrand,
      min_price: minPrice,
      max_price: maxPrice,
      in_stock: inStockOnly ? "true" : "",
      sort: sort !== "newest" ? sort : "",
      page: "1",
      ...overrides,
    };

    for (const [key, value] of Object.entries(nextState)) {
      if (value) {
        params.set(key, value);
      } else {
        params.delete(key);
      }
    }

    startTransition(() => {
      const base = selectedCategorySlug ? `/shop/c/${selectedCategorySlug}` : "/shop";
      router.push(`${base}?${params.toString()}`);
    });
  };

  const handleSearchSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    applyFilters({ q: searchQuery.trim() });
  };

  const handleResetFilters = () => {
    setSearchQuery("");
    setActiveCategory("");
    setActiveBrand("");
    setMinPrice("");
    setMaxPrice("");
    setInStockOnly(false);
    setSort("newest");
    startTransition(() => {
      router.push("/shop");
    });
  };

  // Client-filtered products if initialProducts contains catalog items
  const displayProducts = React.useMemo(() => {
    let list = [...(initialProducts.length > 0 ? initialProducts : SAMPLE_PRODUCTS)];

    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase();
      list = list.filter(
        (p) =>
          p.name.toLowerCase().includes(q) ||
          p.brand.name.toLowerCase().includes(q) ||
          p.category?.name.toLowerCase().includes(q)
      );
    }

    if (activeCategory) {
      list = list.filter((p) => p.category?.slug === activeCategory);
    }

    if (activeBrand) {
      list = list.filter(
        (p) =>
          p.brand.slug === activeBrand ||
          p.brand.name.toLowerCase() === activeBrand.toLowerCase()
      );
    }

    if (inStockOnly) {
      list = list.filter((p) => p.stockStatus === "in_stock");
    }

    if (minPrice && !isNaN(Number(minPrice))) {
      list = list.filter((p) => p.priceFromIdr >= Number(minPrice));
    }

    if (maxPrice && !isNaN(Number(maxPrice))) {
      list = list.filter((p) => p.priceFromIdr <= Number(maxPrice));
    }

    if (sort === "price_asc") {
      list.sort((a, b) => a.priceFromIdr - b.priceFromIdr);
    } else if (sort === "price_desc") {
      list.sort((a, b) => b.priceFromIdr - a.priceFromIdr);
    } else if (sort === "name_asc") {
      list.sort((a, b) => a.name.localeCompare(b.name));
    }

    return list;
  }, [
    initialProducts,
    searchQuery,
    activeCategory,
    activeBrand,
    inStockOnly,
    minPrice,
    maxPrice,
    sort,
  ]);

  const hasActiveFilters = Boolean(
    searchQuery ||
      activeCategory ||
      activeBrand ||
      minPrice ||
      maxPrice ||
      inStockOnly ||
      sort !== "newest"
  );

  return (
    <div className="py-8 md:py-16 bg-bg min-h-screen">
      <div className="gmc-container">
        {/* Page Title & Search Header */}
        <div className="pb-8 border-b border-line flex flex-col md:flex-row md:items-end justify-between gap-6">
          <div>
            <p className="micro-label text-ink-3">KATALOG PERANGKAT IT</p>
            <h1 className="heading-section text-ink mt-2">Koleksi Lengkap.</h1>
          </div>

          {/* Search Bar */}
          <form
            onSubmit={handleSearchSubmit}
            className="w-full md:w-80 relative flex items-center"
          >
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Cari prosesor, GPU, laptop..."
              className="w-full pl-0 pr-8 py-2 bg-transparent border-b border-ink text-sm text-ink placeholder:text-ink-3 focus:outline-none"
            />
            <button
              type="submit"
              aria-label="Cari"
              className="absolute right-0 top-2 text-ink hover:opacity-75 cursor-pointer"
            >
              <Search className="w-4 h-4" />
            </button>
          </form>
        </div>

        {/* Filter Bar / Controls */}
        <div className="py-4 border-b border-line flex flex-wrap items-center justify-between gap-4">
          <div className="flex items-center gap-3">
            {/* Mobile Filter Toggle */}
            <button
              type="button"
              onClick={() => setMobileFilterOpen(true)}
              className="lg:hidden chip-pill inline-flex items-center gap-2"
            >
              <SlidersHorizontal className="w-4 h-4" />
              <span>Filter &amp; Urutkan</span>
            </button>

            {/* Product Counter */}
            <span className="text-xs text-ink-2">
              Menampilkan {displayProducts.length} produk
            </span>
          </div>

          {/* Desktop Sort Dropdown */}
          <div className="hidden lg:flex items-center gap-2">
            <span className="micro-label text-ink-3">URUTKAN:</span>
            <select
              value={sort}
              onChange={(e) => {
                setSort(e.target.value);
                applyFilters({ sort: e.target.value });
              }}
              aria-label="Urutkan produk"
              className="bg-transparent text-xs font-medium text-ink focus:outline-none border-b border-line pb-0.5 cursor-pointer"
            >
              {SORT_OPTIONS.map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {opt.label}
                </option>
              ))}
            </select>
          </div>
        </div>

        {/* Active Filter Tags */}
        {hasActiveFilters && (
          <div className="py-3 flex flex-wrap items-center gap-2 border-b border-line">
            <span className="micro-label text-ink-3 mr-2">FILTER AKTIF:</span>
            {searchQuery && (
              <span className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-panel text-xs text-ink border border-line">
                Kata kunci: &ldquo;{searchQuery}&rdquo;
                <button
                  type="button"
                  onClick={() => {
                    setSearchQuery("");
                    applyFilters({ q: null });
                  }}
                  className="cursor-pointer hover:text-danger"
                >
                  <X className="w-3 h-3" />
                </button>
              </span>
            )}

            {activeCategory && (
              <span className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-panel text-xs text-ink border border-line">
                Kategori: {categories.find((c) => c.slug === activeCategory)?.name || activeCategory}
                <button
                  type="button"
                  onClick={() => {
                    setActiveCategory("");
                    applyFilters({ category: null });
                  }}
                  className="cursor-pointer hover:text-danger"
                >
                  <X className="w-3 h-3" />
                </button>
              </span>
            )}

            {activeBrand && (
              <span className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-panel text-xs text-ink border border-line">
                Brand: {activeBrand}
                <button
                  type="button"
                  onClick={() => {
                    setActiveBrand("");
                    applyFilters({ brand: null });
                  }}
                  className="cursor-pointer hover:text-danger"
                >
                  <X className="w-3 h-3" />
                </button>
              </span>
            )}

            {inStockOnly && (
              <span className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-panel text-xs text-ink border border-line">
                Stok Tersedia
                <button
                  type="button"
                  onClick={() => {
                    setInStockOnly(false);
                    applyFilters({ in_stock: null });
                  }}
                  className="cursor-pointer hover:text-danger"
                >
                  <X className="w-3 h-3" />
                </button>
              </span>
            )}

            <button
              type="button"
              onClick={handleResetFilters}
              className="micro-label text-danger hover:underline ml-2 cursor-pointer"
            >
              Reset Semua
            </button>
          </div>
        )}

        {/* Catalog Main Layout: Desktop Sidebar Filters + Product Grid */}
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-8 pt-8">
          {/* Desktop Left Sidebar */}
          <aside className="hidden lg:block lg:col-span-3 space-y-8 pr-6 border-r border-line">
            {/* Categories */}
            <div>
              <h3 className="micro-label text-ink-3 mb-3">KATEGORI</h3>
              <ul className="space-y-1.5 text-sm">
                <li>
                  <button
                    type="button"
                    onClick={() => {
                      setActiveCategory("");
                      applyFilters({ category: null });
                    }}
                    className={cn(
                      "w-full text-left py-1 hover:text-ink transition-colors cursor-pointer",
                      !activeCategory ? "text-ink font-medium" : "text-ink-2"
                    )}
                  >
                    Semua Kategori
                  </button>
                </li>
                {categories.map((c) => (
                  <li key={c.id}>
                    <button
                      type="button"
                      onClick={() => {
                        setActiveCategory(c.slug);
                        applyFilters({ category: c.slug });
                      }}
                      className={cn(
                        "w-full text-left py-1 hover:text-ink transition-colors cursor-pointer",
                        activeCategory === c.slug
                          ? "text-ink font-medium"
                          : "text-ink-2"
                      )}
                    >
                      {c.name}
                    </button>
                  </li>
                ))}
              </ul>
            </div>

            {/* Price Filter */}
            <div className="pt-6 border-t border-line">
              <h3 className="micro-label text-ink-3 mb-3">RENTANG HARGA (RP)</h3>
              <div className="space-y-3">
                <input
                  type="number"
                  placeholder="Harga minimum"
                  value={minPrice}
                  onChange={(e) => setMinPrice(e.target.value)}
                  className="input-boxed text-xs"
                />
                <input
                  type="number"
                  placeholder="Harga maksimum"
                  value={maxPrice}
                  onChange={(e) => setMaxPrice(e.target.value)}
                  className="input-boxed text-xs"
                />
                <button
                  type="button"
                  onClick={() => applyFilters()}
                  className="btn-pill-secondary w-full h-8 text-[10px]"
                >
                  Terapkan Harga
                </button>
              </div>
            </div>

            {/* In Stock Toggle */}
            <div className="pt-6 border-t border-line">
              <label className="flex items-center gap-2.5 text-sm text-ink cursor-pointer">
                <input
                  type="checkbox"
                  checked={inStockOnly}
                  onChange={(e) => {
                    setInStockOnly(e.target.checked);
                    applyFilters({ in_stock: e.target.checked ? "true" : null });
                  }}
                  className="w-4 h-4 rounded border-line accent-ink cursor-pointer"
                />
                <span>Hanya stok tersedia</span>
              </label>
            </div>

            {/* Brands */}
            {brands.length > 0 && (
              <div className="pt-6 border-t border-line">
                <h3 className="micro-label text-ink-3 mb-3">BRAND</h3>
                <div className="max-h-60 overflow-y-auto space-y-1 pr-2">
                  {brands.map((b) => (
                    <button
                      key={b.id}
                      type="button"
                      onClick={() => {
                        const next = activeBrand === b.slug ? "" : b.slug;
                        setActiveBrand(next);
                        applyFilters({ brand: next || null });
                      }}
                      className={cn(
                        "w-full text-left py-1 text-sm flex items-center justify-between cursor-pointer",
                        activeBrand === b.slug
                          ? "text-ink font-medium"
                          : "text-ink-2 hover:text-ink"
                      )}
                    >
                      <span>{b.name}</span>
                      {activeBrand === b.slug && <Check className="w-3.5 h-3.5 text-ink" />}
                    </button>
                  ))}
                </div>
              </div>
            )}
          </aside>

          {/* Products Grid */}
          <main className="lg:col-span-9">
            {displayProducts.length === 0 ? (
              <div className="py-24 text-center">
                <p className="text-xl text-ink font-normal">
                  Tidak ada produk yang cocok.
                </p>
                <p className="text-sm text-ink-2 mt-2 max-w-sm mx-auto">
                  Coba sesuaikan kata kunci pencarian Anda atau reset filter yang
                  sedang aktif.
                </p>
                <button
                  type="button"
                  onClick={handleResetFilters}
                  className="btn-pill-primary mt-6"
                >
                  Reset Filter
                </button>
              </div>
            ) : (
              <div className="grid grid-cols-2 sm:grid-cols-2 md:grid-cols-3 gap-x-6 gap-y-10">
                {displayProducts.map((product) => (
                  <ProductCard key={product.id} product={product} />
                ))}
              </div>
            )}
          </main>
        </div>
      </div>

      {/* Mobile Filter Modal / Drawer */}
      {mobileFilterOpen && (
        <div className="fixed inset-0 z-50 bg-ink/40 flex justify-end">
          <div className="w-full max-w-sm bg-bg h-full p-6 overflow-y-auto flex flex-col justify-between">
            <div className="space-y-6">
              <div className="flex items-center justify-between pb-4 border-b border-line">
                <span className="micro-label text-ink">FILTER &amp; URUTKAN</span>
                <button
                  type="button"
                  onClick={() => setMobileFilterOpen(false)}
                  className="p-1 text-ink"
                >
                  <X className="w-5 h-5" />
                </button>
              </div>

              {/* Mobile Sort */}
              <div>
                <p className="micro-label text-ink-3 mb-2">URUTKAN</p>
                <div className="space-y-1.5">
                  {SORT_OPTIONS.map((opt) => (
                    <label
                      key={opt.value}
                      className="flex items-center gap-2.5 text-sm py-1 cursor-pointer"
                    >
                      <input
                        type="radio"
                        name="sort-mobile"
                        value={opt.value}
                        checked={sort === opt.value}
                        onChange={(e) => setSort(e.target.value)}
                        className="accent-ink"
                      />
                      <span>{opt.label}</span>
                    </label>
                  ))}
                </div>
              </div>

              {/* Mobile Categories */}
              <div className="pt-4 border-t border-line">
                <p className="micro-label text-ink-3 mb-2">KATEGORI</p>
                <div className="space-y-1 max-h-48 overflow-y-auto">
                  <button
                    type="button"
                    onClick={() => setActiveCategory("")}
                    className={cn(
                      "w-full text-left py-1 text-sm",
                      !activeCategory ? "font-medium text-ink" : "text-ink-2"
                    )}
                  >
                    Semua Kategori
                  </button>
                  {categories.map((c) => (
                    <button
                      key={c.id}
                      type="button"
                      onClick={() => setActiveCategory(c.slug)}
                      className={cn(
                        "w-full text-left py-1 text-sm",
                        activeCategory === c.slug
                          ? "font-medium text-ink"
                          : "text-ink-2"
                      )}
                    >
                      {c.name}
                    </button>
                  ))}
                </div>
              </div>
            </div>

            <div className="pt-6 border-t border-line grid grid-cols-2 gap-3">
              <button
                type="button"
                onClick={() => {
                  handleResetFilters();
                  setMobileFilterOpen(false);
                }}
                className="btn-pill-secondary h-11 text-xs"
              >
                Reset
              </button>
              <button
                type="button"
                onClick={() => {
                  applyFilters();
                  setMobileFilterOpen(false);
                }}
                className="btn-pill-primary h-11 text-xs"
              >
                Terapkan
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
