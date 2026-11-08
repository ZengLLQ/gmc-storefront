"use client";

import React, { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Heart } from "lucide-react";
import type { ProductCardDTO } from "@/lib/api/mappers";
import { useCartStore } from "@/store/useCartStore";
import { useWishlistStore } from "@/store/useWishlistStore";
import { useAuthStore } from "@/store/useAuthStore";
import {
  cn,
  formatIDR,
  makeHardwarePlaceholderSVG,
  STOCK_STATUS_LABELS,
} from "@/lib/format";

export function ProductCard({ product }: { product: ProductCardDTO }) {
  const router = useRouter();
  const addItem = useCartStore((s) => s.addItem);
  const customer = useAuthStore((s) => s.customer);
  const wishlistIds = useWishlistStore((s) => s.productIds);
  const toggleWishlist = useWishlistStore((s) => s.toggle);

  const [hovered, setHovered] = useState(false);

  const isWishlisted = wishlistIds.includes(product.id);
  const primaryImg =
    product.images?.[0]?.url ||
    makeHardwarePlaceholderSVG(product.brand.name, product.name, "primary");
  const secondaryImg =
    product.images?.[1]?.url ||
    makeHardwarePlaceholderSVG(product.brand.name, product.name, "secondary");

  const isOut = product.stockStatus === "out_of_stock";

  const handleQuickAction = (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();

    if (isOut) return;

    if (product.hasVariants || !product.defaultVariantId) {
      router.push(`/shop/${product.slug}`);
      return;
    }

    addItem(
      {
        variantId: product.defaultVariantId,
        productSlug: product.slug,
        sku: product.slug.toUpperCase(),
        name: product.name,
        brand: product.brand.name,
        variantLabel: "",
        imageUrl: primaryImg,
        unitPriceIdr: product.priceFromIdr,
        stockStatus: product.stockStatus,
      },
      true,
      Boolean(customer)
    );
  };

  return (
    <article
      className="group flex flex-col"
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
    >
      {/* Square 1:1 Image Stage with 0px border radius */}
      <div className="relative aspect-square w-full bg-panel overflow-hidden">
        <Link
          href={`/shop/${product.slug}`}
          className="block w-full h-full relative"
        >
          <img
            src={primaryImg}
            alt={product.name}
            loading="lazy"
            className={cn(
              "w-full h-full object-cover transition-opacity duration-500",
              hovered ? "opacity-0" : "opacity-100"
            )}
          />
          <img
            src={secondaryImg}
            alt={`${product.name} — tampak detail`}
            loading="lazy"
            className={cn(
              "w-full h-full object-cover absolute inset-0 transition-opacity duration-500",
              hovered ? "opacity-100" : "opacity-0"
            )}
          />
        </Link>

        {/* Top-left status badge if low stock, preorder, or out of stock */}
        {product.stockStatus !== "in_stock" && (
          <span
            className={cn(
              "absolute top-3 left-3 px-2.5 py-1 rounded-full micro-label",
              isOut
                ? "bg-bg/90 text-ink-3 border border-line"
                : "bg-bg/95 text-ink border border-line"
            )}
          >
            {STOCK_STATUS_LABELS[product.stockStatus] || product.stockStatus}
          </span>
        )}

        {/* Top-right wishlist button */}
        <button
          type="button"
          onClick={(e) => {
            e.preventDefault();
            e.stopPropagation();
            toggleWishlist(product.id, Boolean(customer));
          }}
          aria-label={
            isWishlisted ? "Hapus dari wishlist" : "Simpan ke wishlist"
          }
          className="absolute top-3 right-3 w-8 h-8 rounded-full bg-bg/90 border border-line flex items-center justify-center text-ink hover:bg-ink hover:text-bg transition-colors cursor-pointer"
        >
          <Heart
            className={cn("w-3.5 h-3.5", isWishlisted ? "fill-current" : "")}
          />
        </button>

        {/* Slide-up Add to Cart pill on hover (desktop) / always visible below on mobile */}
        <div className="hidden md:block absolute inset-x-3 bottom-3 translate-y-3 opacity-0 group-hover:translate-y-0 group-hover:opacity-100 transition-all duration-300">
          <button
            type="button"
            disabled={isOut}
            onClick={handleQuickAction}
            className={cn(
              "w-full h-11 rounded-full micro-label transition-opacity cursor-pointer",
              isOut
                ? "bg-bg/90 text-ink-3 border border-line cursor-not-allowed"
                : "bg-btn text-btn-ink hover:opacity-90"
            )}
          >
            {isOut
              ? "STOK HABIS"
              : product.hasVariants
              ? "PILIH VARIAN"
              : "TAMBAH KE KERANJANG"}
          </button>
        </div>
      </div>

      {/* Product Meta */}
      <div className="pt-3.5 flex-1 flex flex-col justify-between">
        <div>
          <p className="micro-label text-ink-3">{product.brand.name}</p>
          <Link
            href={`/shop/${product.slug}`}
            className="mt-1 block text-[15px] font-normal text-ink hover:underline leading-snug line-clamp-2"
          >
            {product.name}
          </Link>
        </div>

        <div className="mt-2 flex flex-col gap-2">
          <div className="flex items-baseline gap-2 flex-wrap">
            <span className="text-[15px] font-medium text-ink">
              {product.priceFromIdr !== product.priceToIdr
                ? `Mulai ${formatIDR(product.priceFromIdr)}`
                : formatIDR(product.priceFromIdr)}
            </span>
            {product.compareAtPriceFromIdr &&
              product.compareAtPriceFromIdr > product.priceFromIdr && (
                <span className="text-xs text-ink-3 line-through">
                  {formatIDR(product.compareAtPriceFromIdr)}
                </span>
              )}
          </div>

          {/* Mobile visible CTA pill */}
          <button
            type="button"
            disabled={isOut}
            onClick={handleQuickAction}
            className={cn(
              "md:hidden w-full h-9 rounded-full micro-label border",
              isOut
                ? "border-line text-ink-3 cursor-not-allowed"
                : "bg-btn text-btn-ink border-btn"
            )}
          >
            {isOut
              ? "STOK HABIS"
              : product.hasVariants
              ? "PILIH VARIAN"
              : "TAMBAH KE KERANJANG"}
          </button>
        </div>
      </div>
    </article>
  );
}
