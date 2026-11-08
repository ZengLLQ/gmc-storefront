"use client";

import React, { useState, Suspense } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { ArrowRight, AlertCircle } from "lucide-react";
import { apiPost } from "@/lib/api/client";
import { useAuthStore } from "@/store/useAuthStore";
import { useCartStore } from "@/store/useCartStore";
import type { UserDTO } from "@/lib/api/mappers";

function LoginForm() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const redirectUrl = searchParams.get("redirect") || "/account";

  const fetchCustomerMe = useAuthStore((s) => s.fetchCustomerMe);
  const mergeWithServer = useCartStore((s) => s.mergeWithServer);

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setErrorMsg(null);
    setLoading(true);

    try {
      await apiPost<{ user: UserDTO }>("/v1/auth/login", {
        email: email.trim(),
        password,
      });

      await fetchCustomerMe();
      await mergeWithServer();
      router.push(redirectUrl);
    } catch (err: unknown) {
      setErrorMsg(
        err instanceof Error
          ? err.message
          : "Login gagal. Periksa kembali email dan password Anda."
      );
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="py-16 md:py-28 bg-bg min-h-[75vh] flex items-center justify-center">
      <div className="w-full max-w-md px-6">
        <div className="text-center mb-8">
          <p className="micro-label text-ink-3">AKUN PELANGGAN</p>
          <h1 className="heading-sub text-ink mt-2">Masuk ke Akun Anda.</h1>
          <p className="text-sm text-ink-2 mt-2">
            Kelola pesanan, simpan alamat pengiriman, dan pantau status servis.
          </p>
        </div>

        {errorMsg && (
          <div className="mb-6 p-4 bg-danger/10 border border-danger/30 text-danger text-xs flex items-center gap-2.5">
            <AlertCircle className="w-4 h-4 shrink-0" />
            <span>{errorMsg}</span>
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="micro-label text-ink-3 block mb-1">
              ALAMAT EMAIL
            </label>
            <input
              type="email"
              required
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="nama@email.com"
              className="input-boxed text-sm"
            />
          </div>

          <div>
            <div className="flex justify-between items-center mb-1">
              <label className="micro-label text-ink-3">PASSWORD</label>
              <Link
                href="/contact"
                className="text-xs text-ink-3 hover:text-ink underline"
              >
                Lupa password?
              </Link>
            </div>
            <input
              type="password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="••••••••"
              className="input-boxed text-sm"
            />
          </div>

          <button
            type="submit"
            disabled={loading}
            className="btn-pill-primary w-full text-center mt-2 inline-flex items-center justify-center gap-2"
          >
            <span>{loading ? "Memverifikasi..." : "Masuk"}</span>
            <ArrowRight className="w-4 h-4" />
          </button>
        </form>

        <div className="mt-8 pt-6 border-t border-line text-center text-sm text-ink-2">
          <span>Belum memiliki akun? </span>
          <Link
            href={`/register${redirectUrl ? `?redirect=${encodeURIComponent(redirectUrl)}` : ""}`}
            className="font-medium text-ink underline hover:opacity-75"
          >
            Daftar sekarang
          </Link>
        </div>
      </div>
    </div>
  );
}

export default function LoginPage() {
  return (
    <Suspense
      fallback={
        <div className="py-24 text-center min-h-[60vh] flex items-center justify-center">
          <p className="text-sm text-ink-3">Memuat halaman masuk...</p>
        </div>
      }
    >
      <LoginForm />
    </Suspense>
  );
}
