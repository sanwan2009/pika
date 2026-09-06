export function hasText(value?: string | null): boolean {
    return value !== null && value !== undefined && value.trim().length > 0;
}
