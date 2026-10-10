import { Link } from 'react-email';
import { BaseTemplate } from '../components/base-template';
import { Button } from '../components/button';
import CardHeader from '../components/card-header';
import { Muted, Paragraph } from '../components/text';
import { colors } from '../components/theme';
import { type SharedProps, sharedPreviewProps, sharedTemplateProps } from '../props';

// YuriID: sent by the forgot-password flow. Reuses the email-verification data shape (VerificationLink = reset link).
interface PasswordResetData {
	userFullName: string;
	verificationLink: string;
}

interface PasswordResetProps extends SharedProps {
	data: PasswordResetData;
}

export const PasswordReset = ({ data, ...props }: PasswordResetProps) => (
	<BaseTemplate {...props} preview={`Reset the password for your ${props.appName} account`}>
		<CardHeader title="Reset your password" />
		<Paragraph>Hello {data.userFullName},</Paragraph>
		<Paragraph>
			Someone (hopefully you) asked to reset the password for your {props.appName} account. This
			link expires in 1 hour and can only be used once. Resetting signs you out on all devices. If
			you did not request this, you can ignore this email.
		</Paragraph>

		<Button href={data.verificationLink}>Reset password</Button>

		<Muted style={{ marginTop: '32px' }}>
			Or if you don't like clicking buttons, open this link:
			<br />
			<Link href={data.verificationLink} style={linkStyle}>
				{data.verificationLink}
			</Link>
		</Muted>
	</BaseTemplate>
);

export default PasswordReset;

const linkStyle = {
	color: colors.mutedForeground,
	textDecoration: 'underline',
	wordBreak: 'break-all' as const
};

PasswordReset.TemplateProps = {
	...sharedTemplateProps,
	data: {
		userFullName: '{{.Data.UserFullName}}',
		verificationLink: '{{.Data.VerificationLink}}'
	}
};

PasswordReset.PreviewProps = {
	...sharedPreviewProps,
	data: {
		userFullName: 'Tim Cook',
		verificationLink: 'https://id.example.com/reset-password?token=abcdefg12345'
	}
};
