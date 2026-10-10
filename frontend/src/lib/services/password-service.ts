import type { User } from '#lib/types/user.type.ts';
import APIService from './api-service';

// YuriID password authentication endpoints
export default class PasswordService extends APIService {
	login = async (identifier: string, password: string) => {
		const res = await this.api.post('/password/login', { identifier, password });
		return res.data as User;
	};

	signup = async (data: { username: string; email: string; password: string }) => {
		await this.api.post('/password/signup', data);
	};

	resendVerification = async (email: string) => {
		await this.api.post('/password/resend-verification', { email });
	};

	verifyEmail = async (token: string) => {
		await this.api.post('/password/verify-email', { token });
	};

	forgot = async (email: string) => {
		await this.api.post('/password/forgot', { email });
	};

	reset = async (token: string, password: string) => {
		await this.api.post('/password/reset', { token, password });
	};

	getOwnStatus = async () => {
		const res = await this.api.get('/users/me/password');
		return res.data as { hasPassword: boolean };
	};

	changeOwn = async (currentPassword: string, newPassword: string) => {
		await this.api.put('/users/me/password', { currentPassword, newPassword });
	};
}
