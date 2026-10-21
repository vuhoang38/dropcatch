// Past this many check times the dots pile into a solid band, so only the line
// shows and hovering marks the point. Grafana does the same.
export const MAX_DOTS = 150

export function showDots(rowCount: number) {
  return rowCount <= MAX_DOTS
}
