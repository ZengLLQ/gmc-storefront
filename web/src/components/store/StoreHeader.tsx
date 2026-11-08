"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { AnimatePresence, motion } from "motion/react";
import { GmcWordmark } from "./GmcWordmark";
import { useCartStore, selectCartTotalCount } from "@/store/useCartStore";
import { useAuthStore } from "@/store/useAuthStore";
import { cn } from "@/lib/format";

const NAV_LINKS = [
  { href: "/", label: "Beranda" },
  { href: "/shop", label: "Shop" },
  { href: "/about", label: "About" },
] as const;

export function StoreHeader({
  storeClosedBanner,
}: {
  storeClosedBanner?: { isOpen: boolean; message: string } | null;
}) {
  const pathname = usePathname();
  const items = useCartStore((s) => s.items);
  const openDrawer = useCartStore((s) => s.openDrawer);
  const fetchCustomerMe = useAuthStore((s) => s.fetchCustomerMe);
  const customerChecked = useAuthStore((s) => s.customerChecked);

  const [mounted, setMounted] = useState(false);
  const [scrolled, setScrolled] = useState(false);
  const [hidden, setHidden] = useState(false);
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const [badgePop, setBadgePop] = useState(false);

  const totalCount = mounted ? selectCartTotalCount(items) : 0;

  useEffect(() => {
    setMounted(true);
    if (!customerChecked) {
      fetchCustomerMe().catch(() => {});
    }
  }, [customerChecked, fetchCustomerMe]);

  useEffect(() => {
    if (!mounted || totalCount === 0) return;
    setBadgePop(true);
    const t = setTimeout(() => setBadgePop(false), 380);
    return () => clearTimeout(t);
  }, [totalCount, mounted]);

  useEffect(() => {
    let lastY = window.scrollY;
    const onScroll = () => {
      const y = window.scrollY;
      setScrolled(y > 16);
      if (y > 140 && y > lastY + 8 && !mobileMenuOpen) {
        setHidden(true);
      } else if (y < lastY - 6 || y <= 140) {
        setHidden(false);
      }
      lastY = y;
    };
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, [mobileMenuOpen]);

  useEffect(() => {
    setMobileMenuOpen(false);
  }, [pathname]);

  return (
    <>
      {storeClosedBanner && !storeClosedBanner.isOpen && (
        <div className="w-full bg-ink text-bg px-4 py-2.5 text-center text-xs tracking-wide">
          {storeClosedBanner.message ||
            "Toko sedang tutup sementara. Anda tetap dapat menjelajahi katalog dan menghubungi kami via WhatsApp."}
        </div>
      )}

      <header
        className={cn(
          "sticky top-0 z-40 w-full transition-all duration-300",
          scrolled
            ? "bg-bg/95 backdrop-blur-sm border-b border-line"
            : "bg-bg border-b border-transparent",
          hidden ? "-translate-y-full" : "translate-y-0"
        )}
      >
        <div className="gmc-container h-16 md:h-20 flex items-center justify-between">
          {/* Left: GMC Wordmark Logo */}
          <Link
            href="/"
            aria-label="Global Mandiri Computer — Beranda"
            className="py-2 focus:outline-none"
          >
            <GmcWordmark />
          </Link>

          {/* Desktop Navigation: ONLY Beranda, Shop, About, Keranjang (n) */}
          <nav
            aria-label="Navigasi Utama"
            className="hidden md:flex items-center gap-10"
          >
            {NAV_LINKS.map((link) => {
              const isActive =
                link.href === "/"
                  ? pathname === "/"
                  : pathname?.startsWith(link.href);
              return (
                <Link
                  key={link.href}
                  href={link.href}
                  className={cn(
                    "text-[15px] transition-colors duration-200",
                    isActive
                      ? "text-ink font-medium"
                      : "text-ink-2 hover:text-ink"
                  )}
                >
                  {link.label}
                </Link>
              );
            })}

            <button
              type="button"
              onClick={openDrawer}
              className="text-[15px] text-ink hover:opacity-75 transition-opacity inline-flex items-center gap-1.5 cursor-pointer"
              aria-label={`Keranjang belanja, ${totalCount} produk`}
            >
              <span>Keranjang</span>
              <span
                className={cn(
                  "inline-flex items-center justify-center min-w-[22px] h-[22px] px-1.5 rounded-full text-xs font-medium bg-ink text-bg transition-transform duration-300",
                  badgePop ? "scale-125" : "scale-100"
                )}
              >
                {totalCount}
              </span>
            </button>
          </nav>

          {/* Mobile Controls */}
          <div className="flex md:hidden items-center gap-4">
            <button
              type="button"
              onClick={openDrawer}
              className="text-sm text-ink inline-flex items-center gap-1.5 py-1"
            >
              <span>Keranjang</span>
              <span className="inline-flex items-center justify-center min-w-[20px] h-[20px] px-1 rounded-full text-[11px] font-medium bg-ink text-bg">
                {totalCount}
              </span>
            </button>

            <button
              type="button"
              onClick={() => setMobileMenuOpen((v) => !v)}
              aria-expanded={mobileMenuOpen}
              aria-label={mobileMenuOpen ? "Tutup menu" : "Buka menu"}
              className="p-2 -mr-2 text-ink focus:outline-none"
            >
              <div className="w-5 flex flex-col gap-1.5">
                <span
                  className={cn(
                    "block h-[1.5px] bg-ink transition-transform duration-300",
                    mobileMenuOpen ? "translate-y-[3.75px] rotate-45" : ""
                  )}
                />
                <span
                  className={cn(
                    "block h-[1.5px] bg-ink transition-transform duration-300",
                    mobileMenuOpen ? "-translate-y-[3.75px] -rotate-45" : ""
                  )}
                />
              </div>
            </button>
          </div>
        </div>
      </header>

      {/* Mobile Full-Screen Cream Overlay */}
      <AnimatePresence>
        {mobileMenuOpen && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.3, ease: [0.22, 1, 0.36, 1] }}
            className="fixed inset-0 z-30 bg-bg pt-24 pb-10 px-6 flex flex-col justify-between md:hidden"
          >
            <nav className="flex flex-col gap-6 mt-6">
              {NAV_LINKS.map((link, i) => (
                <motion.div
                  key={link.href}
                  initial={{ opacity: 0, y: 18 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{
                    delay: 0.06 * i,
                    duration: 0.45,
                    ease: [0.22, 1, 0.36, 1],
                  }}
                >
                  <Link
                    href={link.href}
                    onClick={() => setMobileMenuOpen(false)}
                    className="text-4xl font-normal tracking-tight text-ink block py-1"
                  >
                    {link.label}
                  </Link>
                </motion.div>
              ))}

              <motion.div
                initial={{ opacity: 0, y: 18 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{
                  delay: 0.06 * NAV_LINKS.length,
                  duration: 0.45,
                  ease: [0.22, 1, 0.36, 1],
                }}
              >
                <Link
                  href="/cart"
                  onClick={() => setMobileMenuOpen(false)}
                  className="text-4xl font-normal tracking-tight text-ink inline-flex items-center gap-3 py-1"
                >
                  <span>Keranjang</span>
                  <span className="text-lg px-3 py-0.5 rounded-full bg-ink text-bg">
                    {totalCount}
                  </span>
                </Link>
              </motion.div>
            </nav>

            <div className="border-t border-line pt-6 text-sm text-ink-2 space-y-1">
              <p className="micro-label text-ink-3">LOKASI SHOWROOM</p>
              <p className="text-ink">ITC Mega Grosir Lt. 2 Blok B/7, Surabaya</p>
              <p>WhatsApp: 0822-3255-4674</p>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </>
  );
}
