import clsx from 'clsx';
import React, { useState } from 'react';
import { useHistory } from "react-router-dom";
import styles from './styles.module.css';

const ActionList = [
  {
    title: 'Learn',
    link: 'docs/welcome/',
    description: (
      <>
        Learn the Basics about the Node Software for the 100% decentralized IOTA Network and how it works behind the scenes.
      </>
    ),
  },
  {
    title: 'Build',
    link: 'docs/tutorials/dApp/',
    description: (
      <>
        Follow our tutorials to build your own simple IOTA dApp as GoShimmer plugin.
      </>
    ),
  },
  {
    title: 'Participate',
    link: 'docs/tutorials/setup/',
    description: (
      <>
        Do you want to become part of the network? Learn how to setup your own GoShimmer Node software with our simple guide.
      </>
    ),
  },
];

function Action({ title, link, description}) {
  let [hovering, setHovering] = useState(false);
  let history = useHistory();

  const handleClick = (e) => {
    e.preventDefault();
    history.push(link);
  }

  return (
    <div className='col col--4 margin-vert--md'>
      <div
        className={clsx('card padding--lg')}
        onClick={handleClick}
        onMouseOver={() => setHovering(true)}
        onMouseOut={() => setHovering(false)}
      >
        <div className={clsx(styles.header)}>
          <span className={clsx(styles.headerTitle)}>{title}</span>
          <div href={link} className={clsx(styles.button)}>
            <span className={clsx("material-icons", styles.icon)}>
              navigate_next
            </span>
          </div>
        </div>
        <div className={clsx(
          "headline-stick",
          {
            "size-m": hovering,
            "size-s": !hovering
          }
        )}></div>
        <div className={clsx(styles.body)}>
          {description}
        </div>
      </div>
    </div>
  );
}

function LandingpageActions() {
  return (
    <div className='container padding--xl'>
      <div className="section-header grey text--center margin-bottom--sm" >Get started, right away</div>
      <div className='row'>
        {ActionList.map((props, idx) => (
          <Action key={idx} {...props} />
        ))}
      </div>
    </div>
  );
}

export default LandingpageActions