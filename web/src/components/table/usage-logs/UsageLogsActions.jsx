/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React from 'react';
import { Button, Tag, Space, Skeleton } from '@douyinfe/semi-ui';
import { renderQuota } from '../../../helpers';
import CompactModeToggle from '../../common/ui/CompactModeToggle';
import { useMinimumLoadingTime } from '../../../hooks/common/useMinimumLoadingTime';

const LogsActions = ({
  stat,
  loadingStat,
  showStat,
  compactMode,
  setCompactMode,
  viewMode,
  VIEW_MODE,
  logCount,
  interceptStat,
  getFormValues,
  setInterceptModeQuick,
  exportInterceptCsv,
  t,
}) => {
  const isInterceptView = viewMode === VIEW_MODE.INTERCEPT_DETAILS;
  const showSkeleton = useMinimumLoadingTime(loadingStat);
  const needSkeleton = !showStat || showSkeleton;
  const currentInterceptMode = getFormValues().intercept_mode || '';

  const placeholder = (
    <Space>
      <Skeleton.Title style={{ width: 108, height: 21, borderRadius: 6 }} />
      <Skeleton.Title style={{ width: 65, height: 21, borderRadius: 6 }} />
      <Skeleton.Title style={{ width: 64, height: 21, borderRadius: 6 }} />
    </Space>
  );

  return (
    <div className='flex flex-col md:flex-row justify-between items-start md:items-center gap-2 w-full'>
      {isInterceptView ? (
        <div className='flex flex-col gap-2'>
          <Space wrap>
            {[
              {
                key: '',
                label: t('全部'),
                value: interceptStat.total,
                color: 'grey',
              },
              {
                key: 'normal',
                label: t('正常'),
                value: interceptStat.normal,
                color: 'green',
              },
              {
                key: 'ignore',
                label: 'ignore',
                value: interceptStat.ignore,
                color: 'red',
              },
              {
                key: 'inject',
                label: 'inject',
                value: interceptStat.inject,
                color: 'blue',
              },
              {
                key: 'replace',
                label: 'replace',
                value: interceptStat.replace,
                color: 'orange',
              },
            ].map((item) => (
              <Tag
                key={item.key || 'all'}
                color={currentInterceptMode === item.key ? item.color : 'white'}
                style={{
                  cursor: 'pointer',
                  fontWeight: 500,
                  boxShadow: '0 2px 8px rgba(0, 0, 0, 0.1)',
                  padding: 13,
                  border:
                    currentInterceptMode === item.key
                      ? 'none'
                      : '1px solid var(--semi-color-border)',
                }}
                className='!rounded-lg'
                onClick={() => setInterceptModeQuick(item.key)}
              >
                {item.label}: {Number(item.value || 0).toLocaleString()}
              </Tag>
            ))}
          </Space>
          <Space wrap>
            <Tag
              color='red'
              style={{
                fontWeight: 500,
                boxShadow: '0 2px 8px rgba(0, 0, 0, 0.1)',
                padding: 13,
              }}
              className='!rounded-lg'
            >
              {t('当前结果')}: {Number(logCount || 0).toLocaleString()}
            </Tag>
            <Button type='tertiary' size='small' onClick={exportInterceptCsv}>
              {t('导出 CSV')}
            </Button>
          </Space>
        </div>
      ) : (
        <Skeleton loading={needSkeleton} active placeholder={placeholder}>
          <Space>
            <Tag
              color='blue'
              style={{
                fontWeight: 500,
                boxShadow: '0 2px 8px rgba(0, 0, 0, 0.1)',
                padding: 13,
              }}
              className='!rounded-lg'
            >
              {t('消耗额度')}: {renderQuota(stat.quota)}
            </Tag>
            <Tag
              color='pink'
              style={{
                fontWeight: 500,
                boxShadow: '0 2px 8px rgba(0, 0, 0, 0.1)',
                padding: 13,
              }}
              className='!rounded-lg'
            >
              RPM: {stat.rpm}
            </Tag>
            <Tag
              color='white'
              style={{
                border: 'none',
                boxShadow: '0 2px 8px rgba(0, 0, 0, 0.1)',
                fontWeight: 500,
                padding: 13,
              }}
              className='!rounded-lg'
            >
              TPM: {stat.tpm}
            </Tag>
          </Space>
        </Skeleton>
      )}

      <CompactModeToggle
        compactMode={compactMode}
        setCompactMode={setCompactMode}
        t={t}
      />
    </div>
  );
};

export default LogsActions;
