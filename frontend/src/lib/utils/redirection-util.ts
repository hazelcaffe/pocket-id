import type { User } from '#lib/types/user.type.ts';

// Returns the path to redirect to based on the current path and user authentication status
// If no redirect is needed, it returns null
export function getAuthRedirectPath(url: URL, user: User | null) {
	const path = url.pathname;
	const isSignedIn = !!user;
	const isAdmin = user?.isAdmin;

	const isUnauthenticatedOnlyPath =
		path == '/login' ||
		(path.startsWith('/login/') && path != '/login/alternative/code') ||
		path == '/lc' ||
		path == '/signup' ||
		path == '/signup/setup' ||
		path == '/forgot-password' ||
		path == '/setup' ||
		path.startsWith('/st/');

	const isPublicPath =
		path.startsWith('/lc/') ||
		[
			'/interaction',
			'/interaction/error',
			'/login/alternative/code',
			'/verify-account',
			'/reset-password',
			'/device',
			'/health',
			'/healthz'
		].includes(path);

	const isAdminPath = path == '/settings/admin' || path.startsWith('/settings/admin/');

	if (!isUnauthenticatedOnlyPath && !isPublicPath && !isSignedIn) {
		const redirect = url.pathname + url.search;
		return `/login?redirect=${encodeURIComponent(redirect)}`;
	}

	if (isUnauthenticatedOnlyPath && isSignedIn) {
		return '/settings';
	}

	if (isAdminPath && !isAdmin) {
		return '/settings';
	}

	return null;
}

// YuriID: only same-origin, absolute paths are accepted as post-login redirect targets.
// Rejects absolute URLs, protocol-relative ("//evil.example") and backslash tricks ("/\evil.example"),
// so a crafted ?redirect= link can never bounce a freshly signed-in user to another site.
export function safeRedirectPath(value: string | null | undefined, fallback = '/settings'): string {
	if (!value || !value.startsWith('/') || value.startsWith('//') || value.startsWith('/\\')) {
		return fallback;
	}
	// eslint-disable-next-line no-control-regex
	if (/[\u0000-\u001f\\]/.test(value)) {
		return fallback;
	}
	try {
		const base = 'http://same-origin.invalid';
		const resolved = new URL(value, base);
		if (resolved.origin !== base) return fallback;
		return resolved.pathname + resolved.search + resolved.hash;
	} catch {
		return fallback;
	}
}
