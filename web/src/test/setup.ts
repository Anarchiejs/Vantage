// ECharts a besoin d'un canvas, absent de jsdom : les tests vérifient les options passées, pas le rendu.
HTMLCanvasElement.prototype.getContext = (() => null) as never
class ResizeObserverFactice {
  observe() {}
  unobserve() {}
  disconnect() {}
}
globalThis.ResizeObserver ??= ResizeObserverFactice as never
