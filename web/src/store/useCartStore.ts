"use client";

import { create } from "zustand";
import { persist } from "zustand/middleware";
import { apiPost, apiPut, apiDelete } from "@/lib/api/client";
import type { CartValidationResponseDTO } from "@/lib/api/mappers";

export interface LocalCartItem {
  variantId: string;
  productSlug: string;
  sku: string;
  name: string;
  brand: string;
  variantLabel: string;
  imageUrl?: string | null;
  unitPriceIdr: number;
  quantity: number;
  stock?: number;
  stockStatus?: "in_stock" | "low_stock" | "out_of_stock" | "preorder";
  issue?: string | null;
}

interface CartState {
  items: LocalCartItem[];
  drawerOpen: boolean;
  voucherCode: string;
  discountIdr: number;
  voucherDescription: string | null;
  voucherError: string | null;
  isValidating: boolean;
  openDrawer: () => void;
  closeDrawer: () => void;
  addItem: (
    item: Omit<LocalCartItem, "quantity"> & { quantity?: number },
    openDrawer?: boolean,
    isAuthenticated?: boolean
  ) => void;
  updateQuantity: (
    variantId: string,
    quantity: number,
    isAuthenticated?: boolean
  ) => void;
  removeItem: (variantId: string, isAuthenticated?: boolean) => void;
  clearCart: () => void;
  setVoucherCode: (code: string) => void;
  validateCart: (codeOverride?: string) => Promise<CartValidationResponseDTO | null>;
  mergeWithServer: () => Promise<void>;
}

export const useCartStore = create<CartState>()(
  persist(
    (set, get) => ({
      items: [],
      drawerOpen: false,
      voucherCode: "",
      discountIdr: 0,
      voucherDescription: null,
      voucherError: null,
      isValidating: false,

      openDrawer: () => set({ drawerOpen: true }),
      closeDrawer: () => set({ drawerOpen: false }),

      addItem: (incoming, shouldOpenDrawer = true, isAuthenticated = false) => {
        const qtyToAdd = Math.max(1, incoming.quantity ?? 1);
        const existing = get().items.find(
          (i) => i.variantId === incoming.variantId
        );
        let nextQty = qtyToAdd;
        if (existing) {
          nextQty = Math.min(99, existing.quantity + qtyToAdd);
          set({
            items: get().items.map((i) =>
              i.variantId === incoming.variantId
                ? { ...i, ...incoming, quantity: nextQty, issue: null }
                : i
            ),
            drawerOpen: shouldOpenDrawer ? true : get().drawerOpen,
          });
        } else {
          set({
            items: [
              ...get().items,
              { ...incoming, quantity: nextQty, issue: null },
            ],
            drawerOpen: shouldOpenDrawer ? true : get().drawerOpen,
          });
        }

        if (isAuthenticated) {
          apiPut(`/v1/me/cart/items/${incoming.variantId}`, {
            quantity: nextQty,
          }).catch(() => {});
        }
      },

      updateQuantity: (variantId, quantity, isAuthenticated = false) => {
        if (quantity <= 0) {
          get().removeItem(variantId, isAuthenticated);
          return;
        }
        const clamped = Math.min(99, quantity);
        set({
          items: get().items.map((i) =>
            i.variantId === variantId
              ? { ...i, quantity: clamped, issue: null }
              : i
          ),
        });
        if (isAuthenticated) {
          apiPut(`/v1/me/cart/items/${variantId}`, {
            quantity: clamped,
          }).catch(() => {});
        }
      },

      removeItem: (variantId, isAuthenticated = false) => {
        set({
          items: get().items.filter((i) => i.variantId !== variantId),
        });
        if (isAuthenticated) {
          apiDelete(`/v1/me/cart/items/${variantId}`).catch(() => {});
        }
      },

      clearCart: () =>
        set({
          items: [],
          voucherCode: "",
          discountIdr: 0,
          voucherDescription: null,
          voucherError: null,
        }),

      setVoucherCode: (code) =>
        set({ voucherCode: code.toUpperCase(), voucherError: null }),

      validateCart: async (codeOverride?: string) => {
        const currentItems = get().items;
        if (currentItems.length === 0) {
          set({
            discountIdr: 0,
            voucherDescription: null,
            voucherError: null,
          });
          return null;
        }
        const vCode = (
          codeOverride !== undefined ? codeOverride : get().voucherCode
        ).trim();
        set({ isValidating: true, voucherError: null });
        try {
          const res = await apiPost<CartValidationResponseDTO>(
            "/v1/cart/validate",
            {
              items: currentItems.map((i) => ({
                variantId: i.variantId,
                quantity: i.quantity,
              })),
              voucherCode: vCode || null,
            }
          );

          const mapped: LocalCartItem[] = res.lines.map((line) => ({
            variantId: line.variantId,
            productSlug: line.productSlug,
            sku: line.sku,
            name: line.name,
            brand: line.brand,
            variantLabel: line.variantLabel,
            imageUrl: line.imageUrl,
            unitPriceIdr: line.unitPriceIdr,
            quantity: line.quantity,
            stock: line.stock,
            stockStatus: line.stockStatus,
            issue: line.issue || null,
          }));

          let discount = res.discountIdr || 0;
          let vDesc: string | null = null;
          let vErr: string | null = res.voucherIssue || null;
          if (res.voucher) {
            vDesc = res.voucher.description || `Voucher ${res.voucher.code} aktif`;
          }

          set({
            items: mapped,
            discountIdr: discount,
            voucherDescription: vDesc,
            voucherError: vErr,
            isValidating: false,
          });
          return res;
        } catch {
          set({ isValidating: false });
          return null;
        }
      },

      mergeWithServer: async () => {
        const currentItems = get().items;
        try {
          const res = await apiPost<CartValidationResponseDTO>(
            "/v1/me/cart/merge",
            {
              items: currentItems.map((i) => ({
                variantId: i.variantId,
                quantity: i.quantity,
              })),
            }
          );
          if (res?.lines) {
            set({
              items: res.lines.map((line) => ({
                variantId: line.variantId,
                productSlug: line.productSlug,
                sku: line.sku,
                name: line.name,
                brand: line.brand,
                variantLabel: line.variantLabel,
                imageUrl: line.imageUrl,
                unitPriceIdr: line.unitPriceIdr,
                quantity: line.quantity,
                stock: line.stock,
                stockStatus: line.stockStatus,
                issue: line.issue || null,
              })),
            });
          }
        } catch {
          // Ignore merge error on unauthenticated state
        }
      },
    }),
    {
      name: "gmc.cart.v1",
      partialize: (state) => ({
        items: state.items,
        voucherCode: state.voucherCode,
      }),
    }
  )
);

export function selectCartTotalCount(items: LocalCartItem[]): number {
  return items.reduce((acc, item) => acc + item.quantity, 0);
}

export function selectCartSubtotalIdr(items: LocalCartItem[]): number {
  return items.reduce(
    (acc, item) => acc + item.unitPriceIdr * item.quantity,
    0
  );
}
