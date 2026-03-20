import React, { useEffect, useRef, useState } from 'react';
import { Button, Form, Modal, Space, Typography } from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../../../helpers';

const { Text } = Typography;

const defaultValues = {
  enabled: false,
  mode: 'ignore',
  match_keywords: '',
  inject_prompt: '',
  ignore_response: '',
  replace_rules: '',
};

const RequestInterceptionModal = ({ visible, onCancel, user, onSuccess, t }) => {
  const formApiRef = useRef(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!visible || !user?.id) {
      return;
    }
    const loadPolicy = async () => {
      setLoading(true);
      const res = await API.get(`/api/user/${user.id}/request_interception`);
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        setLoading(false);
        return;
      }
      const policy = data?.request_interception || {};
      formApiRef.current?.setValues({
        enabled: Boolean(policy.enabled),
        mode: policy.mode || 'ignore',
        match_keywords: Array.isArray(policy.match_keywords)
          ? policy.match_keywords.join('\n')
          : '',
        inject_prompt: policy.inject_prompt || '',
        ignore_response: policy.ignore_response || '',
        replace_rules: Array.isArray(policy.replace_rules)
          ? policy.replace_rules.map((rule) => `${rule.from || ''} => ${rule.to || ''}`).join('\n')
          : '',
      });
      setLoading(false);
    };
    loadPolicy();
  }, [visible, user?.id]);

  const parseLines = (value) =>
    String(value || '')
      .split('\n')
      .map((item) => item.trim())
      .filter(Boolean);

  const parseReplaceRules = (value) =>
    parseLines(value)
      .map((line) => {
        const parts = line.split('=>');
        if (parts.length < 2) {
          return null;
        }
        return {
          from: parts[0].trim(),
          to: parts.slice(1).join('=>').trim(),
        };
      })
      .filter((item) => item && item.from);

  const submit = async (values) => {
    if (!user?.id) {
      return;
    }
    setLoading(true);
    const payload = {
      request_interception: {
        enabled: Boolean(values.enabled),
        mode: values.mode,
        match_keywords: parseLines(values.match_keywords),
        inject_prompt: values.inject_prompt || '',
        ignore_response: values.ignore_response || '',
        replace_rules: parseReplaceRules(values.replace_rules),
      },
    };
    const res = await API.put(`/api/user/${user.id}/request_interception`, payload);
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('请求拦截策略已更新'));
      onSuccess?.();
      onCancel?.();
    } else {
      showError(message);
    }
    setLoading(false);
  };

  return (
    <Modal
      title={t('用户请求拦截')}
      visible={visible}
      onCancel={onCancel}
      onOk={() => formApiRef.current?.submitForm()}
      confirmLoading={loading}
      width={720}
    >
      <Form initValues={defaultValues} getFormApi={(api) => (formApiRef.current = api)} onSubmit={submit}>
        {({ values }) => (
          <div className='space-y-3'>
            <Form.Switch field='enabled' label={t('启用拦截')} />

            <Form.Select field='mode' label={t('拦截模式')}>
              <Form.Select.Option value='ignore'>{t('忽视请求')}</Form.Select.Option>
              <Form.Select.Option value='inject'>{t('注入前后文')}</Form.Select.Option>
              <Form.Select.Option value='replace'>{t('关键词替换')}</Form.Select.Option>
            </Form.Select>

            <Form.TextArea
              field='match_keywords'
              label={t('触发关键词')}
              placeholder={t('每行一个关键词；留空表示对该用户所有请求生效')}
              rows={4}
            />

            {values.mode === 'inject' && (
              <Form.TextArea
                field='inject_prompt'
                label={t('注入内容')}
                placeholder={t('命中后会把这段内容注入到请求上下文中')}
                rows={6}
              />
            )}

            {values.mode === 'ignore' && (
              <Form.TextArea
                field='ignore_response'
                label={t('拦截提示')}
                placeholder={t('返回给该用户的固定提示语')}
                rows={4}
              />
            )}

            {values.mode === 'replace' && (
              <Form.TextArea
                field='replace_rules'
                label={t('替换规则')}
                placeholder={t('每行一条，格式：原词 => 新词')}
                rows={6}
              />
            )}

            <Space vertical align='start' spacing='tight'>
              <Text type='tertiary'>{t('说明：忽视请求会直接本地拒绝，不会消耗上游额度。')}</Text>
              <Text type='tertiary'>{t('注入前后文当前优先支持聊天类请求；关键词替换会替换文本内容中的命中词。')}</Text>
            </Space>

            <div className='flex justify-end'>
              <Button theme='borderless' onClick={onCancel}>
                {t('关闭')}
              </Button>
            </div>
          </div>
        )}
      </Form>
    </Modal>
  );
};

export default RequestInterceptionModal;
