'use client'

import './globals.css'
import { useState, useEffect } from 'react'
import Header from '@/components/Header'
import Sidebar from '@/components/Sidebar'

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  const [darkMode, setDarkMode] = useState(true)
  const [sidebarOpen, setSidebarOpen] = useState(true)

  useEffect(() => {
    const saved = localStorage.getItem('darkMode')
    if (saved !== null) setDarkMode(JSON.parse(saved))
  }, [])

  useEffect(() => {
    localStorage.setItem('darkMode', JSON.stringify(darkMode))
  }, [darkMode])

  return (
    <html lang="en" className={darkMode ? 'dark' : 'light'}>
      <body className="min-h-screen bg-gray-900 dark:bg-gray-900 text-white">
        <Header
          darkMode={darkMode}
          onToggleDarkMode={() => setDarkMode(!darkMode)}
        />
        <div className="flex pt-14">
          <Sidebar open={sidebarOpen} onToggle={() => setSidebarOpen(!sidebarOpen)} />
          <main className={`flex-1 p-6 transition-all ${sidebarOpen ? 'ml-64' : 'ml-16'}`}>
            {children}
          </main>
        </div>
      </body>
    </html>
  )
}
