import { safeRedirectPath } from '#lib/utils/redirection-util.ts';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ url }) => {
	return {
		redirect: safeRedirectPath(url.searchParams.get('redirect'))
	};
};
