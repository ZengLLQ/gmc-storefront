"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import { Trash2, Plus, Minus, ArrowRight, ShieldCheck } from "lucide-react";
import {
  useCartStore,
  selectCartSubtotalIdr,
  selectCartTotalCount,
} from "@/store/useCartStore";
import { useAuthStore } from "@/store/useAuthStore";
import { formatIDR, makeHardwarePlaceholderSVG } from "@/lib/format";
import { ProductCard } from "@/components/store/ProductCard";
import { SAMPLE_PRODUCTS } from "@/lib/data/sampleProducts";

export default function CartPage() {
  const items = useCartStore((s) => s.items);
  const updateQuantity = useCartStore((s) => s.updateQuantity);
  const removeItem = useCartStore((s) => s.removeItem);
  const customer = useAuthStore((s) => s.customer);

  const [mounted, setMounted] = useState(false);
  useEffect(() => {
    setMounted(true);
  }, []);

  if (!mounted) {
    return (
      <div className="py-24 text-center min-h-[60vh] flex items-center justify-center">
        <p className="text-sm text-ink-3">Memuat keranjang...</p>
      </div>
    );
  }

  const subtotal = selectCartSubtotalIdr(items);
  const count = selectCartTotalCount(items);
  const isAuth = Boolean(customer);

  return (
    <div className="py-10 md:py-20 bg-bg min-h-[75vh]">
      <div className="gmc-container">
        {/* Header */}
        <div className="pb-8 border-b border-line flex items-baseline justify-between">
          <div>
            <p className="micro-label text-ink-3">KERANJANG BELANJA</p>
            <h1 className="heading-section text-ink mt-2">Pesanan Anda.</h1>
          </div>
          <span className="text-sm text-ink-2">({count} produk)</span>
        </div>

        {items.length === 0 ? (
          <div className="py-24 text-center max-w-md mx-auto">
            <p className="text-2xl font-light text-ink mb-2">
              Keranjang belanja Anda kosong.
            </p>
            <p className="text-sm text-ink-2 mb-8">
              Pilih komponen PC, laptop, monitor, atau perlengkapan jaringan
              terbaik kami di katalog toko.
            </p>
            <Link href="/shop" className="btn-pill-primary">
              Mulai Belanja Sekarang
            </Link>
          </div>
        ) : (
          <div className="grid grid-cols-1 lg:grid-cols-12 gap-12 pt-8">
            {/* Left: Cart Items List */}
            <div className="lg:col-span-8 divide-y divide-line">
              {items.map((item) => {
                const img =
                  item.imageUrl ||
                  makeHardwarePlaceholderSVG(item.brand, item.name);
                return (
                  <div
                    key={item.variantId}
                    className="py-6 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-6"
                  >
                    <div className="flex items-center gap-4">
                      <Link
                        href={`/shop/${item.productSlug}`}
                        className="w-20 h-20 bg-panel shrink-0 border border-line"
                      >
                        <img
                          src={img}
                          alt={item.name}
                          className="w-full h-full object-contain p-2"
                        />
                      </Link>

                      <div>
                        <p className="micro-label text-ink-3">{item.brand}</p>
                        <Link
                          href={`/shop/${item.productSlug}`}
                          className="text-base font-normal text-ink hover:underline line-clamp-1"
                        >
                          {item.name}
                        </Link>
                        {item.variantLabel && (
                          <p className="text-xs text-ink-2 mt-0.5">
                            Varian: {item.variantLabel}
                          </p>
                        )}
                        <p className="text-sm font-medium text-ink mt-1">
                          {formatIDR(item.unitPriceIdr)}
                        </p>
                      </div>
                    </div>

                    <div className="flex items-center justify-between sm:justify-end gap-6 w-full sm:w-auto">
                      {/* Quantity Stepper */}
                      <div className="inline-flex items-center border border-line rounded-full h-9 px-3 gap-3">
                        <button
                          type="button"
                          onClick={() =>
                            updateQuantity(
                              item.variantId,
                              item.quantity - 1,
                              isAuth
                            )
                          }
                          aria-label="Kurangi jumlah"
                          className="text-ink-2 hover:text-ink cursor-pointer"
                        >
                          <Minus className="w-3.5 h-3.5" />
                        </button>
                        <span className="text-xs font-medium min-w-[20px] text-center">
                          {item.quantity}
                        </span>
                        <button
                          type="button"
                          onClick={() =>
                            updateQuantity(
                              item.variantId,
                              item.quantity + 1,
                              isAuth
                            )
                          }
                          aria-label="Tambah jumlah"
                          className="text-ink-2 hover:text-ink cursor-pointer"
                        >
                          <Plus className="w-3.5 h-3.5" />
                        </button>
                      </div>

                      <span className="text-base font-medium text-ink min-w-[100px] text-right">
                        {formatIDR(item.unitPriceIdr * item.quantity)}
                      </span>

                      <button
                        type="button"
                        onClick={() => removeItem(item.variantId, isAuth)}
                        aria-label="Hapus produk"
                        className="p-1.5 text-ink-3 hover:text-danger transition-colors cursor-pointer"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </div>
                );
              })}
            </div>

            {/* Right: Order Summary */}
            <div className="lg:col-span-4">
              <div className="bg-panel border border-line p-6 md:p-8 space-y-6">
                <h2 className="micro-label text-ink">RINGKASAN PESANAN</h2>

                <div className="space-y-3 text-sm text-ink-2 border-b border-line pb-4">
                  <div className="flex justify-between">
                    <span>Subtotal</span>
                    <span className="text-ink font-medium">
                      {formatIDR(subtotal)}
                    </span>
                  </div>
                  <div className="flex justify-between text-xs text-ink-3">
                    <span>Pengiriman</span>
                    <span>Dihitung saat checkout</span>
                  </div>
                  <div className="text-xs text-ink-3 pt-1">
                    *Tersedia opsi ambil gratis di toko fisik ITC Mega Grosir Surabaya.
                  </div>
                </div>

                <div className="flex justify-between items-baseline">
                  <span className="text-base font-medium text-ink">Total</span>
                  <span className="text-2xl font-medium text-ink">
                    {formatIDR(subtotal)}
                  </span>
                </div>

                <Link
                  href="/checkout"
                  className="btn-pill-primary w-full text-center"
                >
                  Lanjut ke Checkout
                </Link>

                <div className="pt-2 text-center">
                  <Link
                    href="/shop"
                    className="micro-label text-ink hover:underline inline-flex items-center gap-1"
                  >
                    Lanjutkan Belanja Katalog →
                  </Link>
                </div>

                <div className="pt-4 border-t border-line text-xs text-ink-3 space-y-2">
                  <div className="flex items-center gap-2">
                    <ShieldCheck className="w-4 h-4 text-ink shrink-0" />
                    <span>Transaksi aman &amp; garansi distributor resmi</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Recommended Products */}
        <div className="mt-24 pt-12 border-t border-line">
          <div className="flex items-end justify-between pb-8">
            <div>
              <p className="micro-label text-ink-3">REKOMENDASI HARDWARE</p>
              <h3 className="heading-sub text-ink mt-1">
                Lengkapi Kebutuhan Setup Anda.
              </h3>
            </div>
            <Link
              href="/shop"
              className="micro-label text-ink hover:underline hidden sm:block"
            >
              Lihat Katalog Lengkap →
            </Link>
          </div>

          <div className="grid grid-cols-2 md:grid-cols-4 gap-6">
            {SAMPLE_PRODUCTS.slice(0, 4).map((p) => (
              <ProductCard key={p.id} product={p} />
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
