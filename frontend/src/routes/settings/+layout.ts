import PasswordService from '#lib/services/password-service.ts';
import StorageService from '#lib/services/storage-service.ts';
import VersionService from '#lib/services/version-service.ts';
import WebAuthnService from '#lib/services/webauthn-service.ts';
import type { AppVersionInformation } from '#lib/types/application-configuration.type.ts';
import { redirect } from '@sveltejs/kit';
import type { LayoutLoad } from './$types';

export const load: LayoutLoad = async ({ url }) => {
	const versionService = new VersionService();
	const storageService = new StorageService();
	const webauthnService = new WebAuthnService();
	const passwordService = new PasswordService();

	const currentVersion = versionService.getCurrentVersion();

	const [newestVersion, sqliteStorageWarning, passkeys, hasPassword] = await Promise.all([
		versionService.getNewestVersion().catch(() => null),
		storageService.getSqliteStorageWarning().catch(() => false),
		webauthnService.listCredentials(),
		passwordService
			.getOwnStatus()
			.then((s) => s.hasPassword)
			.catch(() => false)
	]);

	// If newestVersion is empty, it means the check is disabled or failed.
	// In this case, we assume the version is up to date.
	const isUpToDate =
		newestVersion === null || newestVersion === '' || newestVersion === currentVersion;

	const versionInformation: AppVersionInformation = {
		currentVersion: versionService.getCurrentVersion(),
		newestVersion,
		isUpToDate
	};

	const skipPasskeySetup =
		parseInt(localStorage.getItem('skip-passkey-setup-until') ?? '0') > Date.now();

	// YuriID: users with a password are not forced to add a passkey (it stays optional)
	if (
		!skipPasskeySetup &&
		!hasPassword &&
		passkeys.length === 0 &&
		url.pathname !== '/signup/add-passkey'
	) {
		redirect(303, '/signup/add-passkey');
	}

	return {
		versionInformation,
		sqliteStorageWarning,
		passkeys,
		hasPassword
	};
};
