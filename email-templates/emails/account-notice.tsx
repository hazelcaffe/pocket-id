import { BaseTemplate } from '../components/base-template';
import { Button } from '../components/button';
import CardHeader from '../components/card-header';
import { Paragraph } from '../components/text';
import { type SharedProps, sharedPreviewProps, sharedTemplateProps } from '../props';

// YuriID: neutral notice sent instead of revealing that an email/username is already registered
// (e.g. "someone tried to sign up with this address"). Message is server-generated plain text.
interface AccountNoticeData {
	userFullName: string;
	message: string;
	verificationLink: string;
}

interface AccountNoticeProps extends SharedProps {
	data: AccountNoticeData;
}

export const AccountNotice = ({ data, ...props }: AccountNoticeProps) => (
	<BaseTemplate {...props} preview={`About your ${props.appName} account`}>
		<CardHeader title="About your account" />
		<Paragraph>Hello {data.userFullName},</Paragraph>
		<Paragraph>{data.message}</Paragraph>

		<Button href={data.verificationLink}>Open {props.appName}</Button>
	</BaseTemplate>
);

export default AccountNotice;

AccountNotice.TemplateProps = {
	...sharedTemplateProps,
	data: {
		userFullName: '{{.Data.UserFullName}}',
		message: '{{.Data.Message}}',
		verificationLink: '{{.Data.VerificationLink}}'
	}
};

AccountNotice.PreviewProps = {
	...sharedPreviewProps,
	data: {
		userFullName: 'Tim Cook',
		message:
			'Someone tried to create a new account with this email address. If that was you, sign in instead or reset your password.',
		verificationLink: 'https://id.example.com/login'
	}
};
