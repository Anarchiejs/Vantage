// ECharts needs a canvas, which jsdom does not provide: tests check the options passed, not the drawing.
HTMLCanvasElement.prototype.getContext = (() => null) as never
class FakeResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}
globalThis.ResizeObserver ??= FakeResizeObserver as never
