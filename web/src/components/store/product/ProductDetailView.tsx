"use client";

import React, { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import {
  Heart,
  Minus,
  Plus,
  Share2,
  ShieldCheck,
  Store,
  Truck,
  MessageSquare,
  Check,
  ChevronLeft,
  ChevronRight,
} from "lucide-react";
import type { ProductDetailDTO, ProductCardDTO } from "@/lib/api/mappers";
import { ProductCard } from "@/components/store/ProductCard";
import { useCartStore } from "@/store/useCartStore";
import { useWishlistStore } from "@/store/useWishlistStore";
import { useAuthStore } from "@/store/useAuthStore";
import {
  cn,
  formatIDR,
  makeHardwarePlaceholderSVG,
  STOCK_STATUS_LABELS,
} from "@/lib/format";
import { SITE_CONFIG } from "@/config/site";

export function ProductDetailView({
  product,
  relatedProducts = [],
}: {
  product: ProductDetailDTO;
  relatedProducts?: ProductCardDTO[];
}) {
  const addItem = useCartStore((s) => s.addItem);
  const openDrawer = useCartStore((s) => s.openDrawer);
  const customer = useAuthStore((s) => s.customer);
  const wishlistIds = useWishlistStore((s) => s.productIds);
  const toggleWishlist = useWishlistStore((s) => s.toggle);

  const isWishlisted = wishlistIds.includes(product.id);

  // Gallery state
  const images = useMemo(() => {
    if (product.images && product.images.length > 0) {
      return product.images.map((img) => img.url);
    }
    return [
      makeHardwarePlaceholderSVG(product.brand.name, product.name, "primary"),
      makeHardwarePlaceholderSVG(product.brand.name, product.name, "secondary"),
    ];
  }, [product]);

  const [activeImgIdx, setActiveImgIdx] = useState(0);

  // Selected variant state
  const [selectedVariantId, setSelectedVariantId] = useState<string>(
    product.variants?.[0]?.id || ""
  );
  const [quantity, setQuantity] = useState(1);
  const [activeTab, setActiveTab] = useState<"desc" | "specs" | "shipping">("desc");
  const [copiedShare, setCopiedShare] = useState(false);

  const activeVariant = useMemo(() => {
    if (!product.variants || product.variants.length === 0) return null;
    return (
      product.variants.find((v) => v.id === selectedVariantId) ||
      product.variants[0]
    );
  }, [product.variants, selectedVariantId]);

  const currentPriceIdr = activeVariant
    ? activeVariant.priceIdr
    : product.priceFromIdr;
  const currentComparePrice = activeVariant
    ? activeVariant.compareAtPriceIdr
    : product.compareAtPriceFromIdr;
  const currentStockStatus = activeVariant
    ? activeVariant.stockStatus
    : product.stockStatus;
  const isOutOfStock = currentStockStatus === "out_of_stock";

  const handleAddToCart = () => {
    if (isOutOfStock) return;
    const variantId = activeVariant?.id || product.id;
    const sku = activeVariant?.sku || product.slug.toUpperCase();

    addItem(
      {
        variantId,
        productSlug: product.slug,
        sku,
        name: product.name,
        brand: product.brand.name,
        variantLabel:
          activeVariant && activeVariant.sku !== product.slug.toUpperCase()
            ? activeVariant.sku
            : "",
        imageUrl: images[0],
        unitPriceIdr: currentPriceIdr,
        stockStatus: currentStockStatus,
        quantity,
      },
      true,
      Boolean(customer)
    );
  };

  const handleShare = () => {
    if (typeof window !== "undefined") {
      navigator.clipboard?.writeText(window.location.href);
      setCopiedShare(true);
      setTimeout(() => setCopiedShare(false), 2000);
    }
  };

  // WhatsApp consultation link
  const [currentUrl, setCurrentUrl] = useState("");
  useEffect(() => {
    if (typeof window !== "undefined") {
      setCurrentUrl(window.location.href);
    }
  }, []);

  const waConsultUrl = `${
    SITE_CONFIG.contact.whatsappUrl
  }?text=${encodeURIComponent(
    `Halo GMC ITC Surabaya, saya ingin konsultasi mengenai ketersediaan dan spesifikasi produk: ${product.name}${currentUrl ? ` (${currentUrl})` : ""}`
  )}`;

  return (
    <div className="py-8 md:py-16 bg-bg min-h-screen">
      <div className="gmc-container">
        {/* Breadcrumb Navigation */}
        <nav
          aria-label="Breadcrumb"
          className="pb-6 border-b border-line flex items-center gap-2 text-xs text-ink-3 overflow-x-auto whitespace-nowrap"
        >
          <Link href="/" className="hover:text-ink transition-colors">
            Beranda
          </Link>
          <span>/</span>
          <Link href="/shop" className="hover:text-ink transition-colors">
            Shop
          </Link>
          {product.category && (
            <>
              <span>/</span>
              <Link
                href={`/shop/c/${product.category.slug}`}
                className="hover:text-ink transition-colors"
              >
                {product.category.name}
              </Link>
            </>
          )}
          <span>/</span>
          <span className="text-ink truncate max-w-xs">{product.name}</span>
        </nav>

        {/* 2-Column Desktop Grid: Gallery & Product Info */}
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-12 pt-8">
          {/* Left Column: Gallery Stage with Peek & Thumbnails */}
          <div className="lg:col-span-7 space-y-4">
            <div className="relative aspect-square w-full bg-panel overflow-hidden border border-line">
              <img
                src={images[activeImgIdx]}
                alt={`${product.name} — tampilan ${activeImgIdx + 1}`}
                className="w-full h-full object-contain p-6 md:p-12 transition-all duration-300"
              />

              {/* Status Badge */}
              {currentStockStatus !== "in_stock" && (
                <span className="absolute top-4 left-4 px-3 py-1 rounded-full micro-label bg-bg/95 border border-line text-ink">
                  {STOCK_STATUS_LABELS[currentStockStatus] || currentStockStatus}
                </span>
              )}

              {/* Prev / Next Carousel Buttons */}
              {images.length > 1 && (
                <>
                  <button
                    type="button"
                    onClick={() =>
                      setActiveImgIdx((prev) =>
                        prev === 0 ? images.length - 1 : prev - 1
                      )
                    }
                    aria-label="Gambar sebelumnya"
                    className="absolute left-4 top-1/2 -translate-y-1/2 w-9 h-9 rounded-full bg-bg/90 border border-line flex items-center justify-center text-ink hover:bg-ink hover:text-bg transition-colors cursor-pointer"
                  >
                    <ChevronLeft className="w-4 h-4" />
                  </button>
                  <button
                    type="button"
                    onClick={() =>
                      setActiveImgIdx((prev) => (prev + 1) % images.length)
                    }
                    aria-label="Gambar berikutnya"
                    className="absolute right-4 top-1/2 -translate-y-1/2 w-9 h-9 rounded-full bg-bg/90 border border-line flex items-center justify-center text-ink hover:bg-ink hover:text-bg transition-colors cursor-pointer"
                  >
                    <ChevronRight className="w-4 h-4" />
                  </button>
                </>
              )}
            </div>

            {/* Thumbnail Snap Strip */}
            {images.length > 1 && (
              <div className="flex items-center gap-3 overflow-x-auto pb-2">
                {images.map((img, idx) => (
                  <button
                    key={`${img}-${idx}`}
                    type="button"
                    onClick={() => setActiveImgIdx(idx)}
                    className={cn(
                      "w-20 h-20 bg-panel border shrink-0 p-1.5 transition-all cursor-pointer",
                      idx === activeImgIdx
                        ? "border-ink"
                        : "border-line opacity-60 hover:opacity-100"
                    )}
                  >
                    <img
                      src={img}
                      alt={`Thumbnail ${idx + 1}`}
                      className="w-full h-full object-contain"
                    />
                  </button>
                ))}
              </div>
            )}
          </div>

          {/* Right Column: Details, Variants, CTAs */}
          <div className="lg:col-span-5 flex flex-col justify-between space-y-6">
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <Link
                  href={`/shop?brand=${product.brand.slug}`}
                  className="micro-label text-ink-3 hover:text-ink transition-colors"
                >
                  {product.brand.name}
                </Link>

                <div className="flex items-center gap-2">
                  <button
                    type="button"
                    onClick={handleShare}
                    aria-label="Bagikan link produk"
                    className="p-2 text-ink-2 hover:text-ink transition-colors cursor-pointer"
                    title={copiedShare ? "Link tersalin!" : "Salin link produk"}
                  >
                    {copiedShare ? (
                      <span className="text-xs text-success flex items-center gap-1">
                        <Check className="w-3.5 h-3.5" /> Tersalin
                      </span>
                    ) : (
                      <Share2 className="w-4 h-4" />
                    )}
                  </button>
                  <button
                    type="button"
                    onClick={() => toggleWishlist(product.id, Boolean(customer))}
                    aria-label={
                      isWishlisted ? "Hapus dari wishlist" : "Simpan ke wishlist"
                    }
                    className="p-2 text-ink-2 hover:text-ink transition-colors cursor-pointer"
                  >
                    <Heart
                      className={cn(
                        "w-4 h-4",
                        isWishlisted ? "fill-current text-ink" : ""
                      )}
                    />
                  </button>
                </div>
              </div>

              <h1 className="heading-sub text-ink font-normal">{product.name}</h1>

              {/* Price */}
              <div className="flex items-baseline gap-3 pt-2">
                <span className="text-2xl md:text-3xl font-medium text-ink">
                  {formatIDR(currentPriceIdr)}
                </span>
                {currentComparePrice && currentComparePrice > currentPriceIdr && (
                  <span className="text-sm text-ink-3 line-through">
                    {formatIDR(currentComparePrice)}
                  </span>
                )}
              </div>

              {/* Short Summary */}
              {product.summary && (
                <p className="text-sm text-ink-2 leading-relaxed pt-2 border-t border-line">
                  {product.summary}
                </p>
              )}

              {/* Variant Selector */}
              {product.hasVariants && product.variants && product.variants.length > 1 && (
                <div className="pt-4 border-t border-line space-y-3">
                  <p className="micro-label text-ink-3">PILIH VARIAN / SPESIFIKASI</p>
                  <div className="flex flex-wrap gap-2">
                    {product.variants.map((v) => {
                      const isSelected = v.id === selectedVariantId;
                      return (
                        <button
                          key={v.id}
                          type="button"
                          onClick={() => setSelectedVariantId(v.id)}
                          className={cn(
                            "px-4 py-2 rounded-full text-xs font-medium border transition-colors cursor-pointer",
                            isSelected
                              ? "bg-ink text-bg border-ink"
                              : "bg-bg text-ink border-line hover:border-ink"
                          )}
                        >
                          {v.sku} ({formatIDR(v.priceIdr)})
                        </button>
                      );
                    })}
                  </div>
                </div>
              )}

              {/* Quantity Stepper & Add to Cart Action */}
              <div className="pt-6 border-t border-line space-y-4">
                <div className="flex items-center gap-4">
                  <span className="micro-label text-ink-3">JUMLAH:</span>
                  <div className="inline-flex items-center border border-line rounded-full h-10 px-3 gap-4">
                    <button
                      type="button"
                      disabled={quantity <= 1}
                      onClick={() => setQuantity((q) => Math.max(1, q - 1))}
                      className="text-ink-2 hover:text-ink disabled:opacity-30 cursor-pointer"
                    >
                      <Minus className="w-3.5 h-3.5" />
                    </button>
                    <span className="text-sm font-medium min-w-[20px] text-center">
                      {quantity}
                    </span>
                    <button
                      type="button"
                      onClick={() => setQuantity((q) => q + 1)}
                      className="text-ink-2 hover:text-ink cursor-pointer"
                    >
                      <Plus className="w-3.5 h-3.5" />
                    </button>
                  </div>
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-2">
                  <button
                    type="button"
                    disabled={isOutOfStock}
                    onClick={handleAddToCart}
                    className={cn(
                      "btn-pill-primary w-full text-center",
                      isOutOfStock ? "opacity-40 cursor-not-allowed" : ""
                    )}
                  >
                    {isOutOfStock ? "STOK HABIS" : "TAMBAH KE KERANJANG"}
                  </button>

                  <a
                    href={waConsultUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="btn-pill-secondary w-full text-center inline-flex items-center justify-center gap-1.5"
                  >
                    <MessageSquare className="w-4 h-4" />
                    <span>Tanya via WhatsApp</span>
                  </a>
                </div>
              </div>

              {/* Trust & Guarantee Badges */}
              <div className="pt-6 border-t border-line space-y-3 text-xs text-ink-2">
                <div className="flex items-center gap-2.5">
                  <ShieldCheck className="w-4 h-4 text-ink shrink-0" />
                  <span>
                    Produk 100% Original dari Distributor Resmi di Indonesia.
                  </span>
                </div>
                <div className="flex items-center gap-2.5">
                  <Store className="w-4 h-4 text-ink shrink-0" />
                  <span>
                    Bisa diambil langsung di showroom ITC Mega Grosir Surabaya Lt. 2 Blok B/7.
                  </span>
                </div>
                <div className="flex items-center gap-2.5">
                  <Truck className="w-4 h-4 text-ink shrink-0" />
                  <span>
                    Pengiriman aman dengan proteksi bubble wrap tebal dan asuransi.
                  </span>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Tabbed Specification & Information Section */}
        <div className="mt-20 pt-10 border-t border-line">
          {/* Tab Selector */}
          <div className="flex items-center gap-8 border-b border-line">
            <button
              type="button"
              onClick={() => setActiveTab("desc")}
              className={cn(
                "pb-4 text-sm font-medium tracking-wide transition-colors cursor-pointer relative",
                activeTab === "desc"
                  ? "text-ink"
                  : "text-ink-3 hover:text-ink"
              )}
            >
              PRODUK &amp; DESKRIPSI
              {activeTab === "desc" && (
                <span className="absolute bottom-0 inset-x-0 h-0.5 bg-ink" />
              )}
            </button>

            <button
              type="button"
              onClick={() => setActiveTab("specs")}
              className={cn(
                "pb-4 text-sm font-medium tracking-wide transition-colors cursor-pointer relative",
                activeTab === "specs"
                  ? "text-ink"
                  : "text-ink-3 hover:text-ink"
              )}
            >
              SPESIFIKASI TEKNIS
              {activeTab === "specs" && (
                <span className="absolute bottom-0 inset-x-0 h-0.5 bg-ink" />
              )}
            </button>

            <button
              type="button"
              onClick={() => setActiveTab("shipping")}
              className={cn(
                "pb-4 text-sm font-medium tracking-wide transition-colors cursor-pointer relative",
                activeTab === "shipping"
                  ? "text-ink"
                  : "text-ink-3 hover:text-ink"
              )}
            >
              GARANSI &amp; PENGIRIMAN
              {activeTab === "shipping" && (
                <span className="absolute bottom-0 inset-x-0 h-0.5 bg-ink" />
              )}
            </button>
          </div>

          {/* Tab Content */}
          <div className="py-8 max-w-4xl">
            {activeTab === "desc" && (
              <div className="space-y-6 text-sm md:text-base text-ink-2 leading-relaxed">
                <p>{product.description || product.summary}</p>
                {product.highlights && product.highlights.length > 0 && (
                  <div className="pt-4 space-y-2">
                    <p className="micro-label text-ink">KEUNGGULAN UTAMA</p>
                    <ul className="list-disc pl-5 space-y-1.5 text-ink">
                      {product.highlights.map((h, i) => (
                        <li key={i}>{h}</li>
                      ))}
                    </ul>
                  </div>
                )}
              </div>
            )}

            {activeTab === "specs" && (
              <div className="border border-line divide-y divide-line">
                {product.specs && Object.keys(product.specs).length > 0 ? (
                  Object.entries(product.specs).map(([key, val]) => (
                    <div
                      key={key}
                      className="grid grid-cols-1 sm:grid-cols-3 p-3.5 text-sm"
                    >
                      <span className="font-medium text-ink">{key}</span>
                      <span className="sm:col-span-2 text-ink-2 mt-0.5 sm:mt-0">
                        {val}
                      </span>
                    </div>
                  ))
                ) : (
                  <div className="p-6 text-sm text-ink-3 text-center">
                    Spesifikasi standar distributor resmi tersedia pada kemasan unit.
                  </div>
                )}
              </div>
            )}

            {activeTab === "shipping" && (
              <div className="space-y-6 text-sm text-ink-2 leading-relaxed">
                <div>
                  <h4 className="font-medium text-ink mb-1">Ketentuan Garansi</h4>
                  <p>
                    Semua produk yang dijual di Global Mandiri Computer memiliki
                    garansi resmi distributor Indonesia (1 hingga 5 tahun sesuai
                    ketentuan brand). Klaim garansi dapat dibantu melalui showroom
                    kami di ITC Mega Grosir Surabaya.
                  </p>
                </div>
                <div>
                  <h4 className="font-medium text-ink mb-1">Metode Pengiriman</h4>
                  <ul className="list-disc pl-5 space-y-1">
                    <li>
                      <strong>Ambil di Toko:</strong> Gratis biaya, siap diambil
                      di ITC Mega Grosir Lantai 2 Blok B/7 Surabaya pada hari yang sama.
                    </li>
                    <li>
                      <strong>Kurir Instan:</strong> Tersedia untuk wilayah Surabaya,
                      Sidoarjo, dan Gresik.
                    </li>
                    <li>
                      <strong>Ekspedisi Reguler &amp; Cargo:</strong> JNE, J&amp;T,
                      SiCepat, atau Baraka Sarana Tama dengan packing kayu opsional
                      untuk monitor dan PC rakitan.
                    </li>
                  </ul>
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Cross-Sell / Related Products Section */}
        {relatedProducts.length > 0 && (
          <div className="mt-20 pt-12 border-t border-line">
            <div className="flex items-end justify-between pb-8">
              <div>
                <p className="micro-label text-ink-3">REKOMENDASI</p>
                <h3 className="heading-sub text-ink mt-1">Produk Terkait.</h3>
              </div>
              <Link
                href="/shop"
                className="micro-label text-ink hover:underline hidden sm:block"
              >
                Lihat Semua Katalog →
              </Link>
            </div>

            <div className="grid grid-cols-2 sm:grid-cols-2 md:grid-cols-4 gap-6">
              {relatedProducts.slice(0, 4).map((p) => (
                <ProductCard key={p.id} product={p} />
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Mobile Sticky Bottom Action Bar */}
      <div className="lg:hidden fixed bottom-0 inset-x-0 z-40 bg-bg/95 backdrop-blur-md border-t border-line p-4 flex items-center justify-between gap-4">
        <div className="min-w-0">
          <p className="text-xs text-ink truncate font-medium">{product.name}</p>
          <p className="text-sm font-semibold text-ink">
            {formatIDR(currentPriceIdr)}
          </p>
        </div>
        <button
          type="button"
          disabled={isOutOfStock}
          onClick={handleAddToCart}
          className="btn-pill-primary h-10 px-5 text-[11px] shrink-0"
        >
          {isOutOfStock ? "HABIS" : "BELI SEKARANG"}
        </button>
      </div>

      {/* JSON-LD Schema.org Structured Data */}
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{
          __html: JSON.stringify({
            "@context": "https://schema.org/",
            "@type": "Product",
            name: product.name,
            description: product.summary || product.description,
            brand: {
              "@type": "Brand",
              name: product.brand.name,
            },
            offers: {
              "@type": "Offer",
              priceCurrency: "IDR",
              price: currentPriceIdr,
              availability:
                product.stockStatus === "in_stock"
                  ? "https://schema.org/InStock"
                  : "https://schema.org/OutOfStock",
              url: typeof window !== "undefined" ? window.location.href : "",
            },
          }),
        }}
      />
    </div>
  );
}
