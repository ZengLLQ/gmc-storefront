"use client";

import { create } from "zustand";
import { persist } from "zustand/middleware";
import { apiDelete, apiPut } from "@/lib/api/client";
import type { ProductCardDTO } from "@/lib/api/mappers";

interface WishlistState {
  productIds: string[];
  toggle: (productId: string, isAuthenticated?: boolean) => void;
  has: (productId: string) => boolean;
  setFromServer: (ids: string[]) => void;
}

export const useWishlistStore = create<WishlistState>()(
  persist(
    (set, get) => ({
      productIds: [],
      toggle: (productId, isAuthenticated = false) => {
        const exists = get().productIds.includes(productId);
        const next = exists
          ? get().productIds.filter((id) => id !== productId)
          : [...get().productIds, productId];
        set({ productIds: next });

        if (isAuthenticated) {
          if (exists) {
            apiDelete(`/v1/me/wishlist/${productId}`).catch(() => {});
          } else {
            apiPut(`/v1/me/wishlist/${productId}`).catch(() => {});
          }
        }
      },
      has: (productId) => get().productIds.includes(productId),
      setFromServer: (ids) => {
        const merged = Array.from(new Set([...get().productIds, ...ids]));
        set({ productIds: merged });
      },
    }),
    {
      name: "gmc.wishlist.v1",
    }
  )
);

interface RecentlyViewedState {
  products: ProductCardDTO[];
  recordProduct: (product: ProductCardDTO) => void;
}

export const useRecentlyViewedStore = create<RecentlyViewedState>()(
  persist(
    (set, get) => ({
      products: [],
      recordProduct: (product) => {
        const filtered = get().products.filter((p) => p.id !== product.id);
        set({ products: [product, ...filtered].slice(0, 8) });
      },
    }),
    {
      name: "gmc.recently.v1",
    }
  )
);
