"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  Package,
  MapPin,
  User,
  Heart,
  LogOut,
  ChevronRight,
  ExternalLink,
  Printer,
} from "lucide-react";
import { useAuthStore } from "@/store/useAuthStore";
import { useWishlistStore } from "@/store/useWishlistStore";
import { apiGet } from "@/lib/api/client";
import { formatIDR, cn } from "@/lib/format";
import type { OrderSummaryDTO, AddressDTO } from "@/lib/api/mappers";

export default function AccountPage() {
  const router = useRouter();
  const customer = useAuthStore((s) => s.customer);
  const customerChecked = useAuthStore((s) => s.customerChecked);
  const logoutCustomer = useAuthStore((s) => s.logoutCustomer);
  const wishlistIds = useWishlistStore((s) => s.productIds);

  const [activeTab, setActiveTab] = useState<
    "orders" | "addresses" | "profile" | "wishlist"
  >("orders");

  const [orders, setOrders] = useState<OrderSummaryDTO[]>([]);
  const [addresses, setAddresses] = useState<AddressDTO[]>([]);
  const [loadingOrders, setLoadingOrders] = useState(false);

  useEffect(() => {
    if (customerChecked && !customer) {
      router.push("/login?redirect=/account");
    }
  }, [customerChecked, customer, router]);

  useEffect(() => {
    if (!customer) return;

    if (activeTab === "orders") {
      setLoadingOrders(true);
      apiGet<{ items: OrderSummaryDTO[] }>("/v1/me/orders")
        .then((res) => setOrders(res.items || []))
        .catch(() => setOrders([]))
        .finally(() => setLoadingOrders(false));
    } else if (activeTab === "addresses") {
      apiGet<{ items: AddressDTO[] }>("/v1/me/addresses")
        .then((res) => setAddresses(res.items || []))
        .catch(() => setAddresses([]));
    }
  }, [customer, activeTab]);

  if (!customerChecked || !customer) {
    return (
      <div className="py-24 text-center min-h-[60vh] flex items-center justify-center">
        <p className="text-sm text-ink-3">Memeriksa sesi akun...</p>
      </div>
    );
  }

  const handleLogout = async () => {
    await logoutCustomer();
    router.push("/");
  };

  return (
    <div className="py-10 md:py-20 bg-bg min-h-screen">
      <div className="gmc-container">
        {/* Header */}
        <div className="pb-8 border-b border-line flex flex-col md:flex-row md:items-baseline justify-between gap-4">
          <div>
            <p className="micro-label text-ink-3">PORTAL PELANGGAN</p>
            <h1 className="heading-section text-ink mt-2">
              Halo, {customer.name}.
            </h1>
          </div>
          <button
            type="button"
            onClick={handleLogout}
            className="inline-flex items-center gap-1.5 micro-label text-danger hover:underline cursor-pointer"
          >
            <LogOut className="w-3.5 h-3.5" />
            <span>Keluar dari Akun</span>
          </button>
        </div>

        {/* Account Body: Nav Tabs + Content Pane */}
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-12 pt-8">
          {/* Navigation Sidebar */}
          <aside className="lg:col-span-3 space-y-2">
            <button
              type="button"
              onClick={() => setActiveTab("orders")}
              className={cn(
                "w-full flex items-center justify-between p-3.5 text-sm transition-colors text-left cursor-pointer",
                activeTab === "orders"
                  ? "bg-panel text-ink font-medium border-l-2 border-ink"
                  : "text-ink-2 hover:bg-panel/50 hover:text-ink"
              )}
            >
              <span className="flex items-center gap-2.5">
                <Package className="w-4 h-4" />
                <span>Riwayat Pesanan</span>
              </span>
              <ChevronRight className="w-4 h-4 text-ink-3" />
            </button>

            <button
              type="button"
              onClick={() => setActiveTab("addresses")}
              className={cn(
                "w-full flex items-center justify-between p-3.5 text-sm transition-colors text-left cursor-pointer",
                activeTab === "addresses"
                  ? "bg-panel text-ink font-medium border-l-2 border-ink"
                  : "text-ink-2 hover:bg-panel/50 hover:text-ink"
              )}
            >
              <span className="flex items-center gap-2.5">
                <MapPin className="w-4 h-4" />
                <span>Buku Alamat</span>
              </span>
              <ChevronRight className="w-4 h-4 text-ink-3" />
            </button>

            <button
              type="button"
              onClick={() => setActiveTab("profile")}
              className={cn(
                "w-full flex items-center justify-between p-3.5 text-sm transition-colors text-left cursor-pointer",
                activeTab === "profile"
                  ? "bg-panel text-ink font-medium border-l-2 border-ink"
                  : "text-ink-2 hover:bg-panel/50 hover:text-ink"
              )}
            >
              <span className="flex items-center gap-2.5">
                <User className="w-4 h-4" />
                <span>Profil &amp; Keamanan</span>
              </span>
              <ChevronRight className="w-4 h-4 text-ink-3" />
            </button>

            <button
              type="button"
              onClick={() => setActiveTab("wishlist")}
              className={cn(
                "w-full flex items-center justify-between p-3.5 text-sm transition-colors text-left cursor-pointer",
                activeTab === "wishlist"
                  ? "bg-panel text-ink font-medium border-l-2 border-ink"
                  : "text-ink-2 hover:bg-panel/50 hover:text-ink"
              )}
            >
              <span className="flex items-center gap-2.5">
                <Heart className="w-4 h-4" />
                <span>Wishlist ({wishlistIds.length})</span>
              </span>
              <ChevronRight className="w-4 h-4 text-ink-3" />
            </button>
          </aside>

          {/* Content Pane */}
          <main className="lg:col-span-9">
            {activeTab === "orders" && (
              <div className="space-y-6">
                <div className="flex items-center justify-between pb-4 border-b border-line">
                  <h2 className="micro-label text-ink">DAFTAR PESANAN</h2>
                  <span className="text-xs text-ink-3">
                    {orders.length} pesanan tercatat
                  </span>
                </div>

                {loadingOrders ? (
                  <p className="text-xs text-ink-3 py-8 text-center">
                    Memuat riwayat transaksi...
                  </p>
                ) : orders.length === 0 ? (
                  <div className="py-16 text-center bg-panel border border-line p-6">
                    <Package className="w-10 h-10 text-ink-3 mx-auto mb-3 stroke-[1.5]" />
                    <p className="text-base text-ink font-normal">
                      Belum ada riwayat pesanan.
                    </p>
                    <p className="text-xs text-ink-2 mt-1 mb-6">
                      Mulai jelajahi komponen hardware impian Anda di katalog toko.
                    </p>
                    <Link href="/shop" className="btn-pill-primary">
                      Belanja Sekarang
                    </Link>
                  </div>
                ) : (
                  <div className="border border-line divide-y divide-line">
                    {orders.map((order) => (
                      <div
                        key={order.orderNo}
                        className="p-6 flex flex-col sm:flex-row sm:items-center justify-between gap-4 bg-bg hover:bg-panel/40 transition-colors"
                      >
                        <div>
                          <div className="flex items-center gap-3">
                            <span className="font-mono text-sm font-medium text-ink">
                              {order.orderNo}
                            </span>
                            <span className="px-2.5 py-0.5 rounded-full text-[10px] micro-label bg-panel border border-line">
                              {order.status}
                            </span>
                          </div>
                          <p className="text-xs text-ink-3 mt-1">
                            {new Date(order.createdAt).toLocaleDateString("id-ID", {
                              day: "numeric",
                              month: "long",
                              year: "numeric",
                            })}{" "}
                            · {order.itemCount} produk
                          </p>
                        </div>

                        <div className="flex items-center justify-between sm:justify-end gap-6">
                          <span className="text-base font-medium text-ink">
                            {formatIDR(order.totalIdr)}
                          </span>

                          <Link
                            href={`/checkout/success/${order.orderNo}`}
                            className="btn-pill-secondary h-8 px-4 text-[10px] inline-flex items-center gap-1.5"
                          >
                            <span>Detail</span>
                            <ExternalLink className="w-3 h-3" />
                          </Link>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}

            {activeTab === "addresses" && (
              <div className="space-y-6">
                <div className="flex items-center justify-between pb-4 border-b border-line">
                  <h2 className="micro-label text-ink">ALAMAT PENGIRIMAN</h2>
                  <span className="text-xs text-ink-3">
                    {addresses.length} alamat tersimpan
                  </span>
                </div>

                {addresses.length === 0 ? (
                  <div className="py-16 text-center bg-panel border border-line p-6">
                    <MapPin className="w-10 h-10 text-ink-3 mx-auto mb-3 stroke-[1.5]" />
                    <p className="text-base text-ink">
                      Belum ada alamat pengiriman yang tersimpan.
                    </p>
                    <p className="text-xs text-ink-2 mt-1">
                      Alamat Anda akan otomatis tersimpan saat menyelesaikan checkout.
                    </p>
                  </div>
                ) : (
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    {addresses.map((addr) => (
                      <div
                        key={addr.id}
                        className="p-5 bg-panel border border-line space-y-2 text-sm"
                      >
                        <p className="font-medium text-ink">{addr.recipientName}</p>
                        <p className="text-xs text-ink-2">{addr.phone}</p>
                        <p className="text-xs text-ink">
                          {addr.street}, {addr.city}, {addr.province}{" "}
                          {addr.postalCode}
                        </p>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}

            {activeTab === "profile" && (
              <div className="space-y-6 max-w-lg">
                <div className="pb-4 border-b border-line">
                  <h2 className="micro-label text-ink">DATA DIRI &amp; KEAMANAN</h2>
                </div>

                <div className="space-y-4 text-sm">
                  <div>
                    <label className="micro-label text-ink-3 block mb-1">
                      NAMA LENGKAP
                    </label>
                    <p className="p-3 bg-panel border border-line text-ink">
                      {customer.name}
                    </p>
                  </div>

                  <div>
                    <label className="micro-label text-ink-3 block mb-1">
                      ALAMAT EMAIL
                    </label>
                    <p className="p-3 bg-panel border border-line text-ink">
                      {customer.email}
                    </p>
                  </div>

                  <div>
                    <label className="micro-label text-ink-3 block mb-1">
                      NOMOR WHATSAPP
                    </label>
                    <p className="p-3 bg-panel border border-line text-ink">
                      {customer.phone || "Belum ditambahkan"}
                    </p>
                  </div>
                </div>
              </div>
            )}

            {activeTab === "wishlist" && (
              <div className="space-y-6">
                <div className="flex items-center justify-between pb-4 border-b border-line">
                  <h2 className="micro-label text-ink">PRODUK TERSIMPAN</h2>
                  <span className="text-xs text-ink-3">
                    {wishlistIds.length} produk
                  </span>
                </div>

                {wishlistIds.length === 0 ? (
                  <div className="py-16 text-center bg-panel border border-line p-6">
                    <Heart className="w-10 h-10 text-ink-3 mx-auto mb-3 stroke-[1.5]" />
                    <p className="text-base text-ink">
                      Daftar wishlist Anda masih kosong.
                    </p>
                    <p className="text-xs text-ink-2 mt-1 mb-6">
                      Klik ikon hati pada produk pilihan untuk menyimpannya di sini.
                    </p>
                    <Link href="/shop" className="btn-pill-primary">
                      Eksplorasi Katalog
                    </Link>
                  </div>
                ) : (
                  <div className="text-sm text-ink-2">
                    Tersimpan {wishlistIds.length} produk pilihan. Anda dapat melihatnya
                    langsung di katalog toko.
                  </div>
                )}
              </div>
            )}
          </main>
        </div>
      </div>
    </div>
  );
}
