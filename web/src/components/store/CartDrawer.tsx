"use client";

import React, { useEffect } from "react";
import Link from "next/link";
import { AnimatePresence, motion } from "motion/react";
import { X, Plus, Minus, Trash2 } from "lucide-react";
import {
  useCartStore,
  selectCartSubtotalIdr,
  selectCartTotalCount,
} from "@/store/useCartStore";
import { useAuthStore } from "@/store/useAuthStore";
import { formatIDR, makeHardwarePlaceholderSVG } from "@/lib/format";

export function CartDrawer() {
  const drawerOpen = useCartStore((s) => s.drawerOpen);
  const closeDrawer = useCartStore((s) => s.closeDrawer);
  const items = useCartStore((s) => s.items);
  const updateQuantity = useCartStore((s) => s.updateQuantity);
  const removeItem = useCartStore((s) => s.removeItem);
  const customer = useAuthStore((s) => s.customer);

  const isAuth = Boolean(customer);
  const subtotal = selectCartSubtotalIdr(items);
  const count = selectCartTotalCount(items);

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape" && drawerOpen) {
        closeDrawer();
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [drawerOpen, closeDrawer]);

  return (
    <AnimatePresence>
      {drawerOpen && (
        <>
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.3 }}
            onClick={closeDrawer}
            className="fixed inset-0 z-50 bg-ink/35"
            aria-hidden="true"
          />

          <motion.aside
            role="dialog"
            aria-modal="true"
            aria-label="Keranjang Belanja"
            initial={{ x: "100%" }}
            animate={{ x: 0 }}
            exit={{ x: "100%" }}
            transition={{ duration: 0.45, ease: [0.22, 1, 0.36, 1] }}
            className="fixed top-0 right-0 bottom-0 z-50 w-full max-w-md bg-bg border-l border-line flex flex-col justify-between"
          >
            {/* Header */}
            <div className="px-6 py-5 border-b border-line flex items-center justify-between">
              <div className="flex items-center gap-2.5">
                <span className="micro-label text-ink">KERANJANG BELANJA</span>
                <span className="text-xs text-ink-2">({count} item)</span>
              </div>
              <button
                type="button"
                onClick={closeDrawer}
                aria-label="Tutup keranjang"
                className="p-1.5 text-ink-2 hover:text-ink transition-colors cursor-pointer"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            {/* Items */}
            <div className="flex-1 overflow-y-auto px-6 py-4 divide-y divide-line">
              {items.length === 0 ? (
                <div className="h-full flex flex-col items-center justify-center text-center py-16">
                  <p className="text-lg text-ink mb-2">
                    Keranjang Anda masih kosong.
                  </p>
                  <p className="text-sm text-ink-2 mb-6 max-w-xs">
                    Jelajahi koleksi komponen PC, laptop, monitor, dan perangkat
                    jaringan kami.
                  </p>
                  <Link
                    href="/shop"
                    onClick={closeDrawer}
                    className="btn-pill-primary"
                  >
                    Mulai Belanja
                  </Link>
                </div>
              ) : (
                items.map((item) => {
                  const imgSrc =
                    item.imageUrl ||
                    makeHardwarePlaceholderSVG(item.brand, item.name);
                  return (
                    <div key={item.variantId} className="py-4 flex gap-4">
                      <Link
                        href={`/shop/${item.productSlug}`}
                        onClick={closeDrawer}
                        className="w-20 h-20 bg-panel shrink-0 overflow-hidden block"
                      >
                        <img
                          src={imgSrc}
                          alt={item.name}
                          className="w-full h-full object-cover"
                        />
                      </Link>

                      <div className="flex-1 min-w-0 flex flex-col justify-between">
                        <div>
                          <p className="micro-label text-ink-3">{item.brand}</p>
                          <Link
                            href={`/shop/${item.productSlug}`}
                            onClick={closeDrawer}
                            className="text-sm font-medium text-ink hover:underline line-clamp-1"
                          >
                            {item.name}
                          </Link>
                          {item.variantLabel && (
                            <p className="text-xs text-ink-2 mt-0.5">
                              Varian: {item.variantLabel}
                            </p>
                          )}
                        </div>

                        <div className="flex items-center justify-between mt-3">
                          <div className="inline-flex items-center border border-line rounded-full h-7 px-2 gap-2.5">
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
                              <Minus className="w-3 h-3" />
                            </button>
                            <span className="text-xs font-medium min-w-[16px] text-center">
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
                              <Plus className="w-3 h-3" />
                            </button>
                          </div>

                          <div className="flex items-center gap-3">
                            <span className="text-sm font-medium text-ink">
                              {formatIDR(item.unitPriceIdr * item.quantity)}
                            </span>
                            <button
                              type="button"
                              onClick={() => removeItem(item.variantId, isAuth)}
                              aria-label="Hapus item"
                              className="text-ink-3 hover:text-danger transition-colors cursor-pointer"
                            >
                              <Trash2 className="w-3.5 h-3.5" />
                            </button>
                          </div>
                        </div>
                      </div>
                    </div>
                  );
                })
              )}
            </div>

            {/* Footer */}
            {items.length > 0 && (
              <div className="p-6 border-t border-line bg-bg space-y-4">
                <div className="flex items-center justify-between">
                  <span className="micro-label text-ink-2">SUBTOTAL</span>
                  <span className="text-lg font-medium text-ink">
                    {formatIDR(subtotal)}
                  </span>
                </div>
                <p className="text-xs text-ink-3">
                  Ongkos kirim dan metode pengambilan dipilih saat checkout.
                </p>
                <div className="grid grid-cols-2 gap-3">
                  <Link
                    href="/cart"
                    onClick={closeDrawer}
                    className="btn-pill-secondary w-full text-center"
                  >
                    Lihat Keranjang
                  </Link>
                  <Link
                    href="/checkout"
                    onClick={closeDrawer}
                    className="btn-pill-primary w-full text-center"
                  >
                    Checkout
                  </Link>
                </div>
              </div>
            )}
          </motion.aside>
        </>
      )}
    </AnimatePresence>
  );
}
