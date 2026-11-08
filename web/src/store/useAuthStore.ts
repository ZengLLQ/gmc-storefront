"use client";

import { create } from "zustand";
import { apiGet, apiPost } from "@/lib/api/client";
import type { AdminUserDTO, UserDTO } from "@/lib/api/mappers";

interface AuthState {
  customer: UserDTO | null;
  customerChecked: boolean;
  admin: AdminUserDTO | null;
  adminPermissions: string[];
  adminChecked: boolean;
  fetchCustomerMe: () => Promise<UserDTO | null>;
  setCustomer: (user: UserDTO | null) => void;
  logoutCustomer: () => Promise<void>;
  fetchAdminMe: () => Promise<AdminUserDTO | null>;
  setAdminSession: (user: AdminUserDTO | null, permissions?: string[]) => void;
  logoutAdmin: () => Promise<void>;
  hasAdminPerm: (perm: string) => boolean;
}

export const useAuthStore = create<AuthState>((set, get) => ({
  customer: null,
  customerChecked: false,
  admin: null,
  adminPermissions: [],
  adminChecked: false,

  fetchCustomerMe: async () => {
    try {
      const res = await apiGet<{ user: UserDTO }>("/v1/me");
      set({ customer: res.user, customerChecked: true });
      return res.user;
    } catch {
      set({ customer: null, customerChecked: true });
      return null;
    }
  },

  setCustomer: (user) => set({ customer: user, customerChecked: true }),

  logoutCustomer: async () => {
    try {
      await apiPost("/v1/auth/logout");
    } catch {
      // ignore
    }
    set({ customer: null, customerChecked: true });
  },

  fetchAdminMe: async () => {
    try {
      const res = await apiGet<{ user: AdminUserDTO; permissions: string[] }>(
        "/v1/admin/me"
      );
      set({
        admin: res.user,
        adminPermissions: res.permissions || [],
        adminChecked: true,
      });
      return res.user;
    } catch {
      set({ admin: null, adminPermissions: [], adminChecked: true });
      return null;
    }
  },

  setAdminSession: (user, permissions = []) =>
    set({
      admin: user,
      adminPermissions: permissions,
      adminChecked: true,
    }),

  logoutAdmin: async () => {
    try {
      await apiPost("/v1/admin/auth/logout");
    } catch {
      // ignore
    }
    set({ admin: null, adminPermissions: [], adminChecked: true });
  },

  hasAdminPerm: (perm: string) => {
    const { admin, adminPermissions } = get();
    if (!admin) return false;
    if (admin.role === "admin") return true;
    return adminPermissions.includes(perm);
  },
}));
