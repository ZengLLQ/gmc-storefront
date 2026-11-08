import React from "react";
import type { Metadata } from "next";
import Link from "next/link";
import { ArrowUpRight, ShieldCheck, Cpu, Wrench, Headphones } from "lucide-react";
import { SITE_COPY } from "@/config/copy";
import { SITE_CONFIG } from "@/config/site";
import { BrandMarquee } from "@/components/store/home/BrandMarquee";
import { VisitUs } from "@/components/store/home/VisitUs";

export const metadata: Metadata = {
  title: "Tentang Kami — Global Mandiri Computer ITC Surabaya",
  description:
    "Mengenal Global Mandiri Computer (GMC), pusat pengadaan komputer, laptop, komponen PC, dan Starlink di ITC Mega Grosir Surabaya.",
};

export default function AboutPage() {
  const { about } = SITE_COPY;

  return (
    <div className="bg-bg min-h-screen">
      {/* Hero Section */}
      <section className="py-20 md:py-32 border-b border-line bg-panel">
        <div className="gmc-container">
          <div className="max-w-4xl">
            <span className="micro-label text-ink-3">{about.heroLabel}</span>
            <h1 className="display-hero text-ink mt-4 font-normal">
              {about.heroTitle}
            </h1>
          </div>
        </div>
      </section>

      {/* Story & Philosophy */}
      <section className="py-20 md:py-28 border-b border-line">
        <div className="gmc-container">
          <div className="grid grid-cols-1 lg:grid-cols-12 gap-12 items-start">
            <div className="lg:col-span-5">
              <span className="micro-label text-ink-3">LATAR BELAKANG</span>
              <h2 className="heading-section text-ink mt-2">
                {about.storyTitle}
              </h2>
            </div>

            <div className="lg:col-span-7 space-y-6 text-base md:text-lg text-ink-2 leading-relaxed">
              {about.storyParagraphs.map((para, i) => (
                <p key={i}>{para}</p>
              ))}
              <div className="pt-4">
                <a
                  href={SITE_CONFIG.contact.whatsappUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="btn-pill-primary inline-flex items-center gap-2"
                >
                  <span>Hubungi Tim Kami</span>
                  <ArrowUpRight className="w-4 h-4" />
                </a>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* 4 Process Steps */}
      <section className="py-20 md:py-32 border-b border-line bg-bg">
        <div className="gmc-container">
          <div className="pb-12 border-b border-line flex flex-col md:flex-row md:items-end justify-between gap-6">
            <div>
              <p className="micro-label text-ink-3">ALUR PELAYANAN</p>
              <h2 className="heading-section text-ink mt-2">
                Bagaimana kami bekerja.
              </h2>
            </div>
            <p className="text-sm md:text-base text-ink-2 max-w-md">
              Setiap pesanan komponen dan PC rakitan melalui tahapan validasi yang terstruktur demi keandalan sistem Anda.
            </p>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-8 pt-12">
            {about.processSteps.map((step) => (
              <div
                key={step.step}
                className="p-6 bg-panel border border-line flex flex-col justify-between h-72"
              >
                <div className="flex justify-between items-start">
                  <span className="text-3xl font-light font-mono text-ink-3">
                    {step.step}
                  </span>
                </div>
                <div>
                  <h3 className="text-xl font-normal text-ink mb-2">
                    {step.title}
                  </h3>
                  <p className="text-xs text-ink-2 leading-relaxed">
                    {step.description}
                  </p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Brand Marquee & Visit Us */}
      <BrandMarquee />
      <VisitUs />
    </div>
  );
}
